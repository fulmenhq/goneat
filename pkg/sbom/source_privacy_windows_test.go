package sbom

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

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
