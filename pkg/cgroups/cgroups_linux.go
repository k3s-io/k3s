//go:build linux
// +build linux

package cgroups

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/cgroups"
	cgroupsv2 "github.com/containerd/cgroups/v2"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
)

func Validate() error {
	if cgroups.Mode() == cgroups.Unified {
		return validateCgroupsV2()
	}
	return validateCgroupsV1()
}

func validateCgroupsV1() error {
	cgroups, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return err
	}

	if !strings.Contains(string(cgroups), "cpuset") {
		logrus.Warn(`Failed to find cpuset cgroup, you may need to add "cgroup_enable=cpuset" to your linux cmdline (/boot/cmdline.txt on a Raspberry Pi)`)
	}

	if !strings.Contains(string(cgroups), "memory") {
		msg := "ailed to find memory cgroup, you may need to add \"cgroup_memory=1 cgroup_enable=memory\" to your linux cmdline (/boot/cmdline.txt on a Raspberry Pi)"
		logrus.Error("F" + msg)
		return errors.New("f" + msg)
	}

	return nil
}

func validateCgroupsV2() error {
	manager, err := cgroupsv2.LoadManager("/sys/fs/cgroup", "/")
	if err != nil {
		return err
	}
	controllers, err := manager.RootControllers()
	if err != nil {
		return err
	}
	m := make(map[string]struct{})
	for _, controller := range controllers {
		m[controller] = struct{}{}
	}
	for _, controller := range []string{"cpu", "cpuset", "memory"} {
		if _, ok := m[controller]; !ok {
			return fmt.Errorf("failed to find %s cgroup (v2)", controller)
		}
	}
	return nil
}

func CheckCgroups() (kubeletRoot, runtimeRoot string, controllers map[string]bool) {
	cgroupsModeV2 := cgroups.Mode() == cgroups.Unified
	controllers = make(map[string]bool)

	// For Unified (v2) cgroups we can directly check to see what controllers are mounted
	// under the unified hierarchy.
	if cgroupsModeV2 {
		m, err := cgroupsv2.LoadManager("/sys/fs/cgroup", "/")
		if err != nil {
			return
		}
		enabledControllers, err := m.Controllers()
		if err != nil {
			return
		}
		// Intentionally using an expressionless switch to match the logic below
		for _, controller := range enabledControllers {
			controllers[controller] = true
		}
	}

	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		parts := strings.Split(scan.Text(), ":")
		if len(parts) < 3 {
			continue
		}
		enabledControllers := strings.Split(parts[1], ",")
		// For v1 or hybrid, controller can be a single value {"blkio"}, or a comounted set {"cpu","cpuacct"}
		// For v2, controllers = {""} (only contains a single empty string) so this section is not used.
		for _, controller := range enabledControllers {
			switch {
			case controller == "name=systemd" || cgroupsModeV2:
				// If we detect that we are running under a `.scope` unit with systemd
				// we can assume we are being directly invoked from the command line
				// and thus need to set our kubelet root to something out of the context
				// of `/user.slice` to ensure that `CPUAccounting` and `MemoryAccounting`
				// are enabled, as they are generally disabled by default for `user.slice`
				// Note that we are not setting the `runtimeRoot` as if we are running with
				// `--docker`, we will inadvertently move the cgroup `dockerd` lives in
				//  which is not ideal and causes dockerd to become unmanageable by systemd.
				last := parts[len(parts)-1]
				i := strings.LastIndex(last, ".scope")
				if i > 0 {
					kubeletRoot = "/" + version.Program
				}
			case controller == "cpu":
				// It is common for this to show up multiple times in /sys/fs/cgroup if the controllers are comounted:
				// as "cpu" and "cpuacct", symlinked to the actual hierarchy at "cpu,cpuacct". Unfortunately the order
				// listed in /proc/self/cgroups may not be the same order used in /sys/fs/cgroup, so this check
				// can fail if we use the comma-separated name. Instead, we check for the controller using the symlink.
				p := filepath.Join("/sys/fs/cgroup", controller, parts[2], "cpu.cfs_period_us")
				if _, err := os.Stat(p); err == nil {
					controllers[controller] = true
				}
			default:
				controllers[controller] = true
			}
		}
	}

	// If we're running with v1 and didn't find a scope assigned by systemd, we need to create our own root cgroup to avoid
	// just inheriting from the parent process. The kubelet will take care of moving us into it when we start it up later.
	if kubeletRoot == "" {
		// Examine process ID 1 to see if there is a cgroup assigned to it.
		// When we are not in a container, process 1 is likely to be systemd or some other service manager.
		// It either lives at `/` or `/init.scope` according to https://man7.org/linux/man-pages/man7/systemd.special.7.html
		// When containerized, process 1 will be generally be in a cgroup, otherwise, we may be running in
		// a host PID scenario but we don't support this.
		g, err := os.Open("/proc/1/cgroup")
		if err != nil {
			return
		}
		defer g.Close()
		scan = bufio.NewScanner(g)
		for scan.Scan() {
			parts := strings.Split(scan.Text(), ":")
			if len(parts) < 3 {
				continue
			}
			controllers := strings.Split(parts[1], ",")
			// For v1 or hybrid, controller can be a single value {"blkio"}, or a comounted set {"cpu","cpuacct"}
			// For v2, controllers = {""} (only contains a single empty string)
			for _, controller := range controllers {
				switch {
				case controller == "name=systemd" || cgroupsModeV2:
					last := parts[len(parts)-1]
					if last != "/" && last != "/init.scope" {
						kubeletRoot = "/" + version.Program
						runtimeRoot = "/" + version.Program
					}
				}
			}
		}
	}
	return
}
