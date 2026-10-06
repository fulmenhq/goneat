package sbom

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows does not enforce Unix 0700/0600 through os.Chmod. Protect the new,
// still-empty snapshot root with one current-user ACE inherited by all children.
// FILE_ALL_ACCESS is the file-object full-control mask (not GENERIC_ALL).
const sourceWindowsFullControl = windows.ACCESS_MASK(0x001f01ff)

var sourceReOpenFile = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReOpenFile")

// New children inherit the private DACL, but their default owner may be an
// elevated token's group rather than TokenUser. Reopen the already-created
// object by handle to assign its owner before writing bytes or creating children.
func setSourceChildOwner(file *os.File) (retErr error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() {
		if err := sourceReOpenFile.Find(); err != nil {
			return err
		}
	}
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	handle := windows.InvalidHandle
	var reopenErr error
	controlErr := connection.Control(func(original uintptr) {
		if info.IsDir() {
			// ReOpenFile refuses these os.Root directory handles. Open only
			// the directory object itself relative to its retained handle.
			// No pathname, alternate API, or backup/restore privilege is used.
			name, err := windows.NewNTUnicodeString("")
			if err != nil {
				reopenErr = err
				return
			}
			attributes := windows.OBJECT_ATTRIBUTES{
				RootDirectory: windows.Handle(original),
				ObjectName:    name,
				Attributes:    windows.OBJ_DONT_REPARSE,
			}
			attributes.Length = uint32(unsafe.Sizeof(attributes))
			reopenErr = windows.NtCreateFile(&handle, windows.WRITE_OWNER|windows.READ_CONTROL,
				&attributes, &windows.IO_STATUS_BLOCK{}, nil, 0,
				windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, windows.FILE_OPEN,
				windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT, 0, 0)
			return
		}
		value, _, callErr := sourceReOpenFile.Call(original,
			uintptr(windows.WRITE_OWNER|windows.READ_CONTROL),
			uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE), 0)
		handle = windows.Handle(value)
		if handle == windows.InvalidHandle {
			reopenErr = fmt.Errorf("source SBOM reopen child for owner assignment: %w", callErr)
		}
	})
	if handle != windows.InvalidHandle && handle != 0 {
		defer func() { retErr = errors.Join(retErr, windows.CloseHandle(handle)) }()
	}
	if controlErr != nil {
		return controlErr
	}
	if reopenErr != nil {
		return reopenErr
	}
	if handle == windows.InvalidHandle || handle == 0 {
		return fmt.Errorf("source SBOM owner self-open returned an invalid handle")
	}
	if info.IsDir() {
		// Fail closed if self-open did not return this same non-reparse
		// directory. Query both handles, never a reused pathname.
		var originalInfo, reopenedInfo windows.ByHandleFileInformation
		var identityErr error
		if err := connection.Control(func(original uintptr) {
			identityErr = windows.GetFileInformationByHandle(windows.Handle(original), &originalInfo)
		}); err != nil {
			return err
		}
		if identityErr != nil {
			return identityErr
		}
		if err := windows.GetFileInformationByHandle(handle, &reopenedInfo); err != nil {
			return err
		}
		if reopenedInfo.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
			reopenedInfo.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
			originalInfo.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
			originalInfo.VolumeSerialNumber != reopenedInfo.VolumeSerialNumber ||
			originalInfo.FileIndexHigh != reopenedInfo.FileIndexHigh ||
			originalInfo.FileIndexLow != reopenedInfo.FileIndexLow {
			return fmt.Errorf("source SBOM directory owner self-open changed object identity or reparse state")
		}
	}
	return windows.SetSecurityInfo(handle, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION, user.User.Sid, nil, nil, nil)
}

func makeSourcePrivate(name string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: sourceWindowsFullControl,
		AccessMode:        windows.SET_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(name, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION,
		user.User.Sid, nil, acl, nil)
}

func verifySourcePrivacy(name string, info fs.FileInfo, root bool) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.GetNamedSecurityInfo(name, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("source SBOM cannot verify private Windows ACL at %q: %w", name, err)
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !windows.EqualSid(owner, user.User.Sid) {
		return fmt.Errorf("source SBOM private capture owner changed at %q", name)
	}
	control, _, err := sd.Control()
	if err != nil || (root && control&windows.SE_DACL_PROTECTED == 0) {
		return fmt.Errorf("source SBOM private capture ACL inheritance changed at %q", name)
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil || acl.AceCount != 1 {
		return fmt.Errorf("source SBOM private capture ACL changed at %q", name)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(acl, 0, &ace); err != nil {
		return err
	}
	if ace == nil || ace.Header.AceSize < 20 || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask != sourceWindowsFullControl || ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
		return fmt.Errorf("source SBOM private capture access changed at %q", name)
	}
	// GetAce supplies a Windows-owned validated ACE in the descriptor above;
	// SidStart is the documented variable-sized SID at the end of this structure.
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !sid.IsValid() || !windows.EqualSid(sid, user.User.Sid) {
		return fmt.Errorf("source SBOM private capture trustee changed at %q", name)
	}
	return nil
}
