// +build windows

package cgroups

func Validate() error {
	return nil
}

func CheckCgroups() (kubeletRoot, runtimeRoot string, hasCFS, hasPIDs bool) {
	return "", "", false, false
}
