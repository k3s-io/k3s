package shared

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/robfig/cron.v2"

	"github.com/lxc/lxd/shared/units"
)

type ContainerAction string

const (
	Stop     ContainerAction = "stop"
	Start    ContainerAction = "start"
	Restart  ContainerAction = "restart"
	Freeze   ContainerAction = "freeze"
	Unfreeze ContainerAction = "unfreeze"
)

func IsInt64(value string) error {
	if value == "" {
		return nil
	}

	_, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("Invalid value for an integer: %s", value)
	}

	return nil
}

func IsUint8(value string) error {
	if value == "" {
		return nil
	}

	_, err := strconv.ParseUint(value, 10, 8)
	if err != nil {
		return fmt.Errorf("Invalid value for an integer: %s. Must be between 0 and 255", value)
	}

	return nil
}

func IsUint32(value string) error {
	if value == "" {
		return nil
	}

	_, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return fmt.Errorf("Invalid value for uint32: %s: %v", value, err)
	}

	return nil
}

func IsPriority(value string) error {
	if value == "" {
		return nil
	}

	valueInt, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("Invalid value for an integer: %s", value)
	}

	if valueInt < 0 || valueInt > 10 {
		return fmt.Errorf("Invalid value for a limit '%s'. Must be between 0 and 10", value)
	}

	return nil
}

func IsBool(value string) error {
	if value == "" {
		return nil
	}

	if !StringInSlice(strings.ToLower(value), []string{"true", "false", "yes", "no", "1", "0", "on", "off"}) {
		return fmt.Errorf("Invalid value for a boolean: %s", value)
	}

	return nil
}

func IsOneOf(value string, valid []string) error {
	if value == "" {
		return nil
	}

	if !StringInSlice(value, valid) {
		return fmt.Errorf("Invalid value: %s (not one of %s)", value, valid)
	}

	return nil
}

func IsAny(value string) error {
	return nil
}

func IsNotEmpty(value string) error {
	if value == "" {
		return fmt.Errorf("Required value")
	}

	return nil
}

// IsDeviceID validates string is four lowercase hex characters suitable as Vendor or Device ID.
func IsDeviceID(value string) error {
	if value == "" {
		return nil
	}

	regexHexLc, err := regexp.Compile("^[0-9a-f]+$")
	if err != nil {
		return err
	}

	if len(value) != 4 || !regexHexLc.MatchString(value) {
		return fmt.Errorf("Invalid value, must be four lower case hex characters")
	}

	return nil
}

// IsRootDiskDevice returns true if the given device representation is configured as root disk for
// a container. It typically get passed a specific entry of api.Container.Devices.
func IsRootDiskDevice(device map[string]string) bool {
	// Root disk devices also need a non-empty "pool" property, but we can't check that here
	// because this function is used with clients talking to older servers where there was no
	// concept of a storage pool, and also it is used for migrating from old to new servers.
	// The validation of the non-empty "pool" property is done inside the disk device itself.
	if device["type"] == "disk" && device["path"] == "/" && device["source"] == "" {
		return true
	}

	return false
}

// GetRootDiskDevice returns the container device that is configured as root disk
func GetRootDiskDevice(devices map[string]map[string]string) (string, map[string]string, error) {
	var devName string
	var dev map[string]string

	for n, d := range devices {
		if IsRootDiskDevice(d) {
			if devName != "" {
				return "", nil, fmt.Errorf("More than one root device found")
			}

			devName = n
			dev = d
		}
	}

	if devName != "" {
		return devName, dev, nil
	}

	return "", nil, fmt.Errorf("No root device could be found")
}

// KnownContainerConfigKeys maps all fully defined, well-known config keys
// to an appropriate checker function, which validates whether or not a
// given value is syntactically legal.
var KnownContainerConfigKeys = map[string]func(value string) error{
	"boot.autostart":             IsBool,
	"boot.autostart.delay":       IsInt64,
	"boot.autostart.priority":    IsInt64,
	"boot.stop.priority":         IsInt64,
	"boot.host_shutdown_timeout": IsInt64,

	"limits.cpu": func(value string) error {
		if value == "" {
			return nil
		}

		// Validate the character set
		match, _ := regexp.MatchString("^[-,0-9]*$", value)
		if !match {
			return fmt.Errorf("Invalid CPU limit syntax")
		}

		// Validate first character
		if strings.HasPrefix(value, "-") || strings.HasPrefix(value, ",") {
			return fmt.Errorf("CPU limit can't start with a separator")
		}

		// Validate last character
		if strings.HasSuffix(value, "-") || strings.HasSuffix(value, ",") {
			return fmt.Errorf("CPU limit can't end with a separator")
		}

		return nil
	},
	"limits.cpu.allowance": func(value string) error {
		if value == "" {
			return nil
		}

		if strings.HasSuffix(value, "%") {
			// Percentage based allocation
			_, err := strconv.Atoi(strings.TrimSuffix(value, "%"))
			if err != nil {
				return err
			}

			return nil
		}

		// Time based allocation
		fields := strings.SplitN(value, "/", 2)
		if len(fields) != 2 {
			return fmt.Errorf("Invalid allowance: %s", value)
		}

		_, err := strconv.Atoi(strings.TrimSuffix(fields[0], "ms"))
		if err != nil {
			return err
		}

		_, err = strconv.Atoi(strings.TrimSuffix(fields[1], "ms"))
		if err != nil {
			return err
		}

		return nil
	},
	"limits.cpu.priority": IsPriority,

	"limits.disk.priority": IsPriority,

	"limits.memory": func(value string) error {
		if value == "" {
			return nil
		}

		if strings.HasSuffix(value, "%") {
			_, err := strconv.ParseInt(strings.TrimSuffix(value, "%"), 10, 64)
			if err != nil {
				return err
			}

			return nil
		}

		_, err := units.ParseByteSizeString(value)
		if err != nil {
			return err
		}

		return nil
	},
	"limits.memory.enforce": func(value string) error {
		return IsOneOf(value, []string{"soft", "hard"})
	},
	"limits.memory.swap":          IsBool,
	"limits.memory.swap.priority": IsPriority,

	"limits.network.priority": IsPriority,

	"limits.processes": IsInt64,

	"linux.kernel_modules": IsAny,

	"migration.incremental.memory":            IsBool,
	"migration.incremental.memory.iterations": IsUint32,
	"migration.incremental.memory.goal":       IsUint32,

	"nvidia.runtime":             IsBool,
	"nvidia.driver.capabilities": IsAny,
	"nvidia.require.cuda":        IsAny,
	"nvidia.require.driver":      IsAny,

	"security.nesting":       IsBool,
	"security.privileged":    IsBool,
	"security.devlxd":        IsBool,
	"security.devlxd.images": IsBool,

	"security.protection.delete": IsBool,
	"security.protection.shift":  IsBool,

	"security.idmap.base":     IsUint32,
	"security.idmap.isolated": IsBool,
	"security.idmap.size":     IsUint32,

	"security.syscalls.blacklist_default":       IsBool,
	"security.syscalls.blacklist_compat":        IsBool,
	"security.syscalls.blacklist":               IsAny,
	"security.syscalls.intercept.mknod":         IsBool,
	"security.syscalls.intercept.mount":         IsBool,
	"security.syscalls.intercept.mount.allowed": IsAny,
	"security.syscalls.intercept.mount.shift":   IsBool,
	"security.syscalls.intercept.setxattr":      IsBool,
	"security.syscalls.whitelist":               IsAny,

	"snapshots.schedule": func(value string) error {
		if value == "" {
			return nil
		}

		if len(strings.Split(value, " ")) != 5 {
			return fmt.Errorf("Schedule must be of the form: <minute> <hour> <day-of-month> <month> <day-of-week>")
		}

		_, err := cron.Parse(fmt.Sprintf("* %s", value))
		if err != nil {
			return errors.Wrap(err, "Error parsing schedule")
		}

		return nil
	},
	"snapshots.schedule.stopped": IsBool,
	"snapshots.pattern":          IsAny,
	"snapshots.expiry": func(value string) error {
		// Validate expression
		_, err := GetSnapshotExpiry(time.Time{}, value)
		return err
	},

	// Caller is responsible for full validation of any raw.* value
	"raw.apparmor": IsAny,
	"raw.lxc":      IsAny,
	"raw.seccomp":  IsAny,
	"raw.idmap":    IsAny,

	"volatile.apply_template":   IsAny,
	"volatile.base_image":       IsAny,
	"volatile.last_state.idmap": IsAny,
	"volatile.last_state.power": IsAny,
	"volatile.idmap.base":       IsAny,
	"volatile.idmap.current":    IsAny,
	"volatile.idmap.next":       IsAny,
	"volatile.apply_quota":      IsAny,
}

// ConfigKeyChecker returns a function that will check whether or not
// a provide value is valid for the associate config key.  Returns an
// error if the key is not known.  The checker function only performs
// syntactic checking of the value, semantic and usage checking must
// be done by the caller.  User defined keys are always considered to
// be valid, e.g. user.* and environment.* keys.
func ConfigKeyChecker(key string) (func(value string) error, error) {
	if f, ok := KnownContainerConfigKeys[key]; ok {
		return f, nil
	}

	if strings.HasPrefix(key, "volatile.") {
		if strings.HasSuffix(key, ".hwaddr") {
			return IsAny, nil
		}

		if strings.HasSuffix(key, ".name") {
			return IsAny, nil
		}

		if strings.HasSuffix(key, ".host_name") {
			return IsAny, nil
		}

		if strings.HasSuffix(key, ".mtu") {
			return IsAny, nil
		}

		if strings.HasSuffix(key, ".created") {
			return IsAny, nil
		}

		if strings.HasSuffix(key, ".id") {
			return IsAny, nil
		}

		if strings.HasSuffix(key, ".vlan") {
			return IsAny, nil
		}

		if strings.HasSuffix(key, ".spoofcheck") {
			return IsAny, nil
		}

		if strings.HasSuffix(key, ".apply_quota") {
			return IsAny, nil
		}
	}

	if strings.HasPrefix(key, "environment.") {
		return IsAny, nil
	}

	if strings.HasPrefix(key, "user.") {
		return IsAny, nil
	}

	if strings.HasPrefix(key, "image.") {
		return IsAny, nil
	}

	if strings.HasPrefix(key, "limits.kernel.") &&
		(len(key) > len("limits.kernel.")) {
		return IsAny, nil
	}

	return nil, fmt.Errorf("Unknown configuration key: %s", key)
}

// ContainerGetParentAndSnapshotName returns the parent container name, snapshot
// name, and whether it actually was a snapshot name.
func ContainerGetParentAndSnapshotName(name string) (string, string, bool) {
	fields := strings.SplitN(name, SnapshotDelimiter, 2)
	if len(fields) == 1 {
		return name, "", false
	}

	return fields[0], fields[1], true
}
