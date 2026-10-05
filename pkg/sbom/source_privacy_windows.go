package sbom

import (
	"fmt"
	"io/fs"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows does not enforce Unix 0700/0600 through os.Chmod. Protect the new,
// still-empty snapshot root with one current-user ACE inherited by all children.
// FILE_ALL_ACCESS is the file-object full-control mask (not GENERIC_ALL).
const sourceWindowsFullControl = windows.ACCESS_MASK(0x001f01ff)

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
