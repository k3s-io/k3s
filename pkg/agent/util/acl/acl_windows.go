//go:build windows
// +build windows

package acl

import (
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

// TODO: Remove in favor of the rancher/permissions repository once that is setup

func BuiltinAdministratorsSID() *windows.SID {
	return mustGetSid(windows.WinBuiltinAdministratorsSid)
}

func LocalSystemSID() *windows.SID {
	return mustGetSid(windows.WinLocalSystemSid)
}

func mustGetSid(sidType windows.WELL_KNOWN_SID_TYPE) *windows.SID {
	sid, err := windows.CreateWellKnownSid(sidType)
	if err != nil {
		panic(err)
	}
	return sid
}

// GrantSid creates an EXPLICIT_ACCESS instance granting permissions to the provided SID.
func GrantSid(accessPermissions windows.ACCESS_MASK, sid *windows.SID) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: accessPermissions,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}

// Apply performs both Chmod and Chown at the same time, where the filemode's owner and group will correspond to
// the provided owner and group (or the current owner and group, if they are set to nil)
func Apply(path string, owner *windows.SID, group *windows.SID, access ...windows.EXPLICIT_ACCESS) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	return apply(path, owner, group, access...)
}

// apply performs a Chmod (if owner and group are provided) and sets a custom ACL based on the provided EXPLICIT_ACCESS rules
// To create EXPLICIT_ACCESS rules, see the helper functions in pkg/access
func apply(path string, owner *windows.SID, group *windows.SID, access ...windows.EXPLICIT_ACCESS) error {
	// assemble arguments
	args := securityArgs{
		path:   path,
		owner:  owner,
		group:  group,
		access: access,
	}

	securityInfo := args.ToSecurityInfo()
	if securityInfo == 0 {
		// nothing to change
		return nil
	}
	dacl, err := args.ToDACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		securityInfo,
		owner,
		group,
		dacl,
		nil,
	)
}

type securityArgs struct {
	path string

	owner *windows.SID
	group *windows.SID

	access []windows.EXPLICIT_ACCESS
}

func (a *securityArgs) ToSecurityInfo() windows.SECURITY_INFORMATION {
	var securityInfo windows.SECURITY_INFORMATION

	if a.owner != nil {
		// override owner
		securityInfo |= windows.OWNER_SECURITY_INFORMATION
	}

	if a.group != nil {
		// override group
		securityInfo |= windows.GROUP_SECURITY_INFORMATION
	}

	if len(a.access) != 0 {
		// override DACL
		securityInfo |= windows.DACL_SECURITY_INFORMATION
		securityInfo |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	}

	return securityInfo
}

func (a *securityArgs) ToSecurityAttributes() (*windows.SecurityAttributes, error) {
	// define empty security descriptor
	sd, err := windows.NewSecurityDescriptor()
	if err != nil {
		return nil, err
	}
	err = sd.SetOwner(a.owner, false)
	if err != nil {
		return nil, err
	}
	err = sd.SetGroup(a.group, false)
	if err != nil {
		return nil, err
	}

	// define security attributes using descriptor
	var sa windows.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.SecurityDescriptor = sd

	if len(a.access) == 0 {
		// security attribute should simply inherit parent rules
		sa.InheritHandle = 1
		return &sa, nil
	}

	// apply provided access rules to the DACL
	dacl, err := a.ToDACL()
	if err != nil {
		return nil, err
	}
	err = sd.SetDACL(dacl, true, false)
	if err != nil {
		return nil, err
	}

	// set the protected DACL flag to prevent the DACL of the security descriptor from being modified by inheritable ACEs
	// (i.e. prevent parent folders from modifying this ACL)
	err = sd.SetControl(windows.SE_DACL_PROTECTED, windows.SE_DACL_PROTECTED)
	if err != nil {
		return nil, err
	}

	return &sa, nil
}

func (a *securityArgs) ToDACL() (*windows.ACL, error) {
	if len(a.access) == 0 {
		// No rules were specified
		return nil, nil
	}
	return windows.ACLFromEntries(a.access, nil)
}
