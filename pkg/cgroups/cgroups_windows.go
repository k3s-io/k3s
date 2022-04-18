//go:build windows
// +build windows

package cgroups

func Validate() error {
	return nil
}

func CheckCgroups() (kubeletRoot, runtimeRoot string, controllers map[string]bool) {
	return
}
