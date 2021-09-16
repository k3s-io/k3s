// +build linux,cgo,seccomp

package seccomp

import (
	"errors"
	"fmt"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/seccomp/patchbpf"

	libseccomp "github.com/seccomp/libseccomp-golang"
	"golang.org/x/sys/unix"
)

var (
	actAllow = libseccomp.ActAllow
	actTrap  = libseccomp.ActTrap
	actKill  = libseccomp.ActKill
	actTrace = libseccomp.ActTrace.SetReturnCode(int16(unix.EPERM))
	actLog   = libseccomp.ActLog
	actErrno = libseccomp.ActErrno.SetReturnCode(int16(unix.EPERM))
)

const (
	// Linux system calls can have at most 6 arguments
	syscallMaxArguments int = 6
)

// Filters given syscalls in a container, preventing them from being used
// Started in the container init process, and carried over to all child processes
// Setns calls, however, require a separate invocation, as they are not children
// of the init until they join the namespace
func InitSeccomp(config *configs.Seccomp) error {
	if config == nil {
		return errors.New("cannot initialize Seccomp - nil config passed")
	}

	defaultAction, err := getAction(config.DefaultAction, config.DefaultErrnoRet)
	if err != nil {
		return errors.New("error initializing seccomp - invalid default action")
	}

	filter, err := libseccomp.NewFilter(defaultAction)
	if err != nil {
		return fmt.Errorf("error creating filter: %s", err)
	}

	// Add extra architectures
	for _, arch := range config.Architectures {
		scmpArch, err := libseccomp.GetArchFromString(arch)
		if err != nil {
			return fmt.Errorf("error validating Seccomp architecture: %s", err)
		}
		if err := filter.AddArch(scmpArch); err != nil {
			return fmt.Errorf("error adding architecture to seccomp filter: %s", err)
		}
	}

	// Unset no new privs bit
	if err := filter.SetNoNewPrivsBit(false); err != nil {
		return fmt.Errorf("error setting no new privileges: %s", err)
	}

	// Add a rule for each syscall
	for _, call := range config.Syscalls {
		if call == nil {
			return errors.New("encountered nil syscall while initializing Seccomp")
		}
		if err := matchCall(filter, call, defaultAction); err != nil {
			return err
		}
	}
	if err := patchbpf.PatchAndLoad(config, filter); err != nil {
		return fmt.Errorf("error loading seccomp filter into kernel: %s", err)
	}
	return nil
}

// Convert Libcontainer Action to Libseccomp ScmpAction
func getAction(act configs.Action, errnoRet *uint) (libseccomp.ScmpAction, error) {
	switch act {
	case configs.Kill:
		return actKill, nil
	case configs.Errno:
		if errnoRet != nil {
			return libseccomp.ActErrno.SetReturnCode(int16(*errnoRet)), nil
		}
		return actErrno, nil
	case configs.Trap:
		return actTrap, nil
	case configs.Allow:
		return actAllow, nil
	case configs.Trace:
		if errnoRet != nil {
			return libseccomp.ActTrace.SetReturnCode(int16(*errnoRet)), nil
		}
		return actTrace, nil
	case configs.Log:
		return actLog, nil
	default:
		return libseccomp.ActInvalid, errors.New("invalid action, cannot use in rule")
	}
}

// Convert Libcontainer Operator to Libseccomp ScmpCompareOp
func getOperator(op configs.Operator) (libseccomp.ScmpCompareOp, error) {
	switch op {
	case configs.EqualTo:
		return libseccomp.CompareEqual, nil
	case configs.NotEqualTo:
		return libseccomp.CompareNotEqual, nil
	case configs.GreaterThan:
		return libseccomp.CompareGreater, nil
	case configs.GreaterThanOrEqualTo:
		return libseccomp.CompareGreaterEqual, nil
	case configs.LessThan:
		return libseccomp.CompareLess, nil
	case configs.LessThanOrEqualTo:
		return libseccomp.CompareLessOrEqual, nil
	case configs.MaskEqualTo:
		return libseccomp.CompareMaskedEqual, nil
	default:
		return libseccomp.CompareInvalid, errors.New("invalid operator, cannot use in rule")
	}
}

// Convert Libcontainer Arg to Libseccomp ScmpCondition
func getCondition(arg *configs.Arg) (libseccomp.ScmpCondition, error) {
	cond := libseccomp.ScmpCondition{}

	if arg == nil {
		return cond, errors.New("cannot convert nil to syscall condition")
	}

	op, err := getOperator(arg.Op)
	if err != nil {
		return cond, err
	}

	return libseccomp.MakeCondition(arg.Index, op, arg.Value, arg.ValueTwo)
}

// Add a rule to match a single syscall
func matchCall(filter *libseccomp.ScmpFilter, call *configs.Syscall, defAct libseccomp.ScmpAction) error {
	if call == nil || filter == nil {
		return errors.New("cannot use nil as syscall to block")
	}

	if len(call.Name) == 0 {
		return errors.New("empty string is not a valid syscall")
	}

	// Convert the call's action to the libseccomp equivalent
	callAct, err := getAction(call.Action, call.ErrnoRet)
	if err != nil {
		return fmt.Errorf("action in seccomp profile is invalid: %w", err)
	}
	if callAct == defAct {
		// This rule is redundant, silently skip it
		// to avoid error from AddRule.
		return nil
	}

	// If we can't resolve the syscall, assume it's not supported on this kernel
	// Ignore it, don't error out
	callNum, err := libseccomp.GetSyscallFromName(call.Name)
	if err != nil {
		return nil
	}

	// Unconditional match - just add the rule
	if len(call.Args) == 0 {
		if err := filter.AddRule(callNum, callAct); err != nil {
			return fmt.Errorf("error adding seccomp filter rule for syscall %s: %s", call.Name, err)
		}
	} else {
		// If two or more arguments have the same condition,
		// Revert to old behavior, adding each condition as a separate rule
		argCounts := make([]uint, syscallMaxArguments)
		conditions := []libseccomp.ScmpCondition{}

		for _, cond := range call.Args {
			newCond, err := getCondition(cond)
			if err != nil {
				return fmt.Errorf("error creating seccomp syscall condition for syscall %s: %s", call.Name, err)
			}

			argCounts[cond.Index] += 1

			conditions = append(conditions, newCond)
		}

		hasMultipleArgs := false
		for _, count := range argCounts {
			if count > 1 {
				hasMultipleArgs = true
				break
			}
		}

		if hasMultipleArgs {
			// Revert to old behavior
			// Add each condition attached to a separate rule
			for _, cond := range conditions {
				condArr := []libseccomp.ScmpCondition{cond}

				if err := filter.AddRuleConditional(callNum, callAct, condArr); err != nil {
					return fmt.Errorf("error adding seccomp rule for syscall %s: %s", call.Name, err)
				}
			}
		} else {
			// No conditions share same argument
			// Use new, proper behavior
			if err := filter.AddRuleConditional(callNum, callAct, conditions); err != nil {
				return fmt.Errorf("error adding seccomp rule for syscall %s: %s", call.Name, err)
			}
		}
	}

	return nil
}

// Version returns major, minor, and micro.
func Version() (uint, uint, uint) {
	return libseccomp.GetLibraryVersion()
}
