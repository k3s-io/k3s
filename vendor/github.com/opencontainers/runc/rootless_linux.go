// +build linux

package main

import (
	"os"

	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/urfave/cli"
)

func shouldUseRootlessCgroupManager(context *cli.Context) (bool, error) {
	if context != nil {
		b, err := parseBoolOrAuto(context.GlobalString("rootless"))
		if err != nil {
			return false, err
		}
		// nil b stands for "auto detect"
		if b != nil {
			return *b, nil
		}

		if context.GlobalBool("systemd-cgroup") {
			return false, nil
		}
	}
	if os.Geteuid() != 0 {
		return true, nil
	}
	if !system.RunningInUserNS() {
		// euid == 0 , in the initial ns (i.e. the real root)
		return false, nil
	}
	// euid = 0, in a userns.
	// As we are unaware of cgroups path, we can't determine whether we have the full
	// access to the cgroups path.
	// Either way, we can safely decide to use the rootless cgroups manager.
	return true, nil
}

func shouldHonorXDGRuntimeDir() bool {
	if os.Getenv("XDG_RUNTIME_DIR") == "" {
		return false
	}
	if os.Geteuid() != 0 {
		return true
	}
	if !system.RunningInUserNS() {
		// euid == 0 , in the initial ns (i.e. the real root)
		// in this case, we should use /run/runc and ignore
		// $XDG_RUNTIME_DIR (e.g. /run/user/0) for backward
		// compatibility.
		return false
	}
	// euid = 0, in a userns.
	u, ok := os.LookupEnv("USER")
	return !ok || u != "root"
}
