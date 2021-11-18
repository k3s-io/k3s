package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
)

type SysctlError struct {
	err    string
	option string
	value  int
	fatal  bool
}

// Error return the error as string
func (e *SysctlError) Error() string {
	return fmt.Sprintf("Sysctl %s=%d : %s", e.option, e.value, e.err)
}

// IsFatal was the error fatal and reason to exit kube-router
func (e *SysctlError) IsFatal() bool {
	return e.fatal
}

// SetSysctl sets a sysctl value
func SetSysctl(path string, value int) *SysctlError {
	sysctlPath := fmt.Sprintf("/proc/sys/%s", path)
	if _, err := os.Stat(sysctlPath); err != nil {
		if os.IsNotExist(err) {
			return &SysctlError{"option not found, Does your kernel version support this feature?", path, value, false}
		}
		return &SysctlError{"stat error: " + err.Error(), path, value, true}
	}
	err := ioutil.WriteFile(sysctlPath, []byte(strconv.Itoa(value)), 0640)
	if err != nil {
		return &SysctlError{"could not set due to: " + err.Error(), path, value, true}
	}
	return nil
}
