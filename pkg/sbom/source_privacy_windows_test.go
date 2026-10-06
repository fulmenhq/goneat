package sbom

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func sourceReplacementDenied(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

func TestSourceReplacementDenialClassification(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"access-denied", windows.ERROR_ACCESS_DENIED, true},
		{"sharing-violation", windows.ERROR_SHARING_VIOLATION, true},
		{"wrapped-denial", &os.LinkError{Op: "rename", Err: windows.ERROR_ACCESS_DENIED}, true},
		{"generic-permission", os.ErrPermission, false},
		{"missing-file", windows.ERROR_FILE_NOT_FOUND, false},
		{"generic-error", errors.New("access is denied"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sourceReplacementDenied(tc.err); got != tc.want {
				t.Fatalf("denial classification=%v want=%v for %v", got, tc.want, tc.err)
			}
		})
	}
}

func TestSourceChildOwnerRejectsClosedHandle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := setSourceChildOwner(file); err == nil {
		t.Fatal("owner assignment silently accepted a closed handle")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) != 0 {
		t.Fatalf("failed owner assignment wrote data: %q %v", data, err)
	}
}

func TestSourceChildOwnerErrorStages(t *testing.T) {
	stages := []string{"directory-self-open", "original-identity-query", "reopened-identity-query", "owner-assignment", "owner-handle-close"}
	for _, stage := range stages {
		if got := sourceOwnerStageError(stage, nil); got != nil {
			t.Fatalf("successful stage %s returned %v", stage, got)
		}
		win32 := sourceOwnerStageError(stage, windows.ERROR_ACCESS_DENIED)
		if !errors.Is(win32, windows.ERROR_ACCESS_DENIED) || !strings.Contains(win32.Error(), stage) {
			t.Fatalf("stage %s lost Win32 errno or label: %v", stage, win32)
		}
		status := windows.NTStatus(0xc0000022) // STATUS_ACCESS_DENIED
		nt := sourceOwnerStageError(stage, status)
		var recovered windows.NTStatus
		if !errors.Is(nt, status) || !errors.As(nt, &recovered) || recovered != status || !strings.Contains(nt.Error(), stage) {
			t.Fatalf("stage %s lost NTSTATUS or label: %v", stage, nt)
		}
	}
	primary := sourceOwnerStageError("directory-self-open", windows.ERROR_ACCESS_DENIED)
	closeErr := sourceOwnerStageError("owner-handle-close", windows.ERROR_INVALID_HANDLE)
	joined := errors.Join(primary, closeErr)
	if !errors.Is(joined, windows.ERROR_ACCESS_DENIED) || !errors.Is(joined, windows.ERROR_INVALID_HANDLE) ||
		!strings.Contains(joined.Error(), "directory-self-open") || !strings.Contains(joined.Error(), "owner-handle-close") {
		t.Fatalf("joined close error lost a stage or identity: %v", joined)
	}
}

func TestSourceChildDirectoryOwnerIdentity(t *testing.T) {
	const wantAccess = windows.WRITE_OWNER | windows.READ_CONTROL | windows.FILE_READ_ATTRIBUTES
	if sourceWindowsDirectoryOwnerAccess != wantAccess {
		t.Fatalf("directory owner access=%#x, want only owner/control/read-attributes %#x", sourceWindowsDirectoryOwnerAccess, wantAccess)
	}
	if sourceWindowsDirectoryOwnerAccess != 0x000a0080 {
		t.Fatalf("directory owner access=%#x, want exact mask 0x000a0080", sourceWindowsDirectoryOwnerAccess)
	}
	path := t.TempDir()
	if err := makeSourcePrivate(path); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := root.Mkdir("child", 0o700); err != nil {
		t.Fatal(err)
	}
	child, err := root.Open("child")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := child.Close(); err != nil {
			t.Error(err)
		}
	})
	before, err := child.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := setSourceChildOwner(child); err != nil {
		t.Fatalf("required single native directory self-open/owner assignment: %v", err)
	}
	after, err := child.Stat()
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("owner assignment changed child directory object: %v", err)
	}
	if err := verifySourcePrivacy(filepath.Join(path, "child"), after, false); err != nil {
		t.Fatal(err)
	}
}

func TestSourceCaptureOwnerMutation(t *testing.T) {
	groups, err := windows.GetCurrentProcessToken().GetTokenGroups()
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	var other *windows.SID
	for _, group := range groups.AllGroups() {
		if group.Attributes&windows.SE_GROUP_OWNER != 0 && !windows.EqualSid(group.Sid, user.User.Sid) {
			other = group.Sid
			break
		}
	}
	if other == nil {
		t.Fatal("required owner-tamper fixture needs an assignable non-user owner in the hosted token")
	}
	for _, name := range []string{".", "nested", "nested/file"} {
		t.Run(name, func(t *testing.T) {
			root, parent := t.TempDir(), t.TempDir()
			writeSourceFixture(t, root, "nested/file", "private bytes")
			capture, err := captureSource(context.Background(), root, parent, SourceOptions{}, sourceLimits{entries: 10, bytes: 100})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := capture.cleanup(); err != nil {
					t.Error(err)
				}
			})
			path := filepath.Join(capture.path, filepath.FromSlash(name))
			t.Cleanup(func() {
				if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, user.User.Sid, nil, nil, nil); err != nil {
					t.Error(err)
				}
			})
			if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, other, nil, nil, nil); err != nil {
				t.Fatalf("required owner-tamper fixture: %v", err)
			}
			if err := capture.verify(context.Background()); err == nil || !strings.Contains(err.Error(), "owner changed") {
				t.Fatalf("non-user owner accepted or wrong rejection: %v", err)
			}
		})
	}
}

// Hosted Windows tests use the current token's symlink privilege when present;
// Developer Mode's unprivileged route remains usable when it is absent. The
// required native manifest rejects any resulting symlink-fixture SKIP.
func prepareSourceSymlinkFixture(t *testing.T) {
	t.Helper()
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		t.Logf("symlink privilege token unavailable; attempting unprivileged creation: %v", err)
		return
	}
	t.Cleanup(func() {
		if err := token.Close(); err != nil {
			t.Error(err)
		}
	})
	name, err := windows.UTF16PtrFromString("SeCreateSymbolicLinkPrivilege")
	if err != nil {
		t.Fatal(err)
	}
	var privilege windows.LUID
	if err := windows.LookupPrivilegeValue(nil, name, &privilege); err != nil {
		t.Fatal(err)
	}
	requested := windows.Tokenprivileges{PrivilegeCount: 1}
	requested.Privileges[0] = windows.LUIDAndAttributes{Luid: privilege, Attributes: windows.SE_PRIVILEGE_ENABLED}
	var previous windows.Tokenprivileges
	var size uint32
	if err := windows.AdjustTokenPrivileges(token, false, &requested, uint32(unsafe.Sizeof(previous)), &previous, &size); err != nil {
		t.Logf("symlink privilege unavailable; attempting unprivileged creation: %v", err)
		return
	}
	t.Cleanup(func() {
		if err := windows.AdjustTokenPrivileges(token, false, &previous, 0, nil, nil); err != nil {
			t.Error(err)
		}
	})
}

func TestSourceCapturePrivacyMutation(t *testing.T) {
	root, parent := t.TempDir(), t.TempDir()
	writeSourceFixture(t, root, "file", "private data")
	capture, err := captureSource(context.Background(), root, parent, SourceOptions{}, sourceLimits{entries: 10, bytes: 100})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := capture.cleanup(); err != nil {
			t.Error(err)
		}
	})
	everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	var access []windows.EXPLICIT_ACCESS
	for _, sid := range []*windows.SID{everyone, user.User.Sid} {
		access = append(access, windows.EXPLICIT_ACCESS{
			AccessPermissions: sourceWindowsFullControl,
			AccessMode:        windows.SET_ACCESS,
			Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeValue: windows.TrusteeValueFromSID(sid)},
		})
	}
	acl, err := windows.ACLFromEntries(access, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(capture.path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if err := capture.verify(context.Background()); err == nil {
		t.Fatal("widened private Windows ACL accepted")
	}
}

func TestSourceCaptureUnreadableACL(t *testing.T) {
	root, parent := t.TempDir(), t.TempDir()
	writeSourceFixture(t, root, "bin/unreadable", "excluded private bytes")
	path := filepath.Join(root, "bin", "unreadable")
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	trustee := windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid)}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		{AccessPermissions: windows.FILE_READ_DATA, AccessMode: windows.DENY_ACCESS, Trustee: trustee},
		{AccessPermissions: sourceWindowsFullControl, AccessMode: windows.GRANT_ACCESS, Trustee: trustee},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := makeSourcePrivate(path); err != nil {
			t.Error(err)
		}
	})
	if file, err := os.Open(path); err == nil {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
		t.Fatal("ACL fixture did not deny file reads")
	}
	capture, err := captureSource(context.Background(), root, parent, SourceOptions{}, sourceLimits{entries: 10, bytes: 100})
	if err == nil || capture != nil {
		t.Fatal("unreadable excluded Windows file accepted")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed capture retained private content: %v %v", entries, err)
	}
}
