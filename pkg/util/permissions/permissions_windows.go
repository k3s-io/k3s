//go:build windows
// +build windows

package permissions

import (
	"fmt"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

// IsPrivileged returns an error if the the process is not running as a member of the BUILTIN\Administrators group.
// Ref: https://github.com/kubernetes/kubernetes/pull/96616
func IsPrivileged() error {
	var sid *windows.SID

	// Although this looks scary, it is directly copied from the
	// official windows documentation. The Go API for this is a
	// direct wrap around the official C++ API.
	// Ref: https://docs.microsoft.com/en-us/windows/desktop/api/securitybaseapi/nf-securitybaseapi-checktokenmembership
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		return errors.Wrap(err, "failed to create Windows SID")
	}
	defer windows.FreeSid(sid)

	// Ref: https://github.com/golang/go/issues/28804#issuecomment-438838144
	token := windows.Token(0)

	member, err := token.IsMember(sid)
	if err != nil {
		return errors.Wrap(err, "failed to check group membership")
	}

	if !member {
		return fmt.Errorf("not running as member of BUILTIN\\Administrators group")
	}

	return nil
}
