package cgrouputil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/sys/mountinfo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// EvacuateCgroup2 evacuates cgroup2. Must be called in the parent PID namespace.
//
// When the current process belongs to "/foo" group (visible under "/sys/fs/cgroup/foo") and evac is like "bar",
// - All processes in the "/foo" group are moved to "/foo/bar" group, by writing PIDs into "/sys/fs/cgroup/foo/bar/cgroup.procs"
// - As many controllers as possible are enabled for "/foo/*" groups, by writing "/sys/fs/cgroup/foo/cgroup.subtree_control"
//
// Returns nil when cgroup2 is not enabled.
// Ported from https://github.com/rootless-containers/usernetes/commit/46ad812db7489914897ff8b1774f2fab0efda62b
func EvacuateCgroup2(evac string) error {
	if evac == "" {
		return errors.New("got empty evacuation group name")
	}
	if strings.Contains(evac, "/") {
		return errors.Errorf("unexpected evacuation group name %q: must not contain \"/\"", evac)
	}

	mountpoint := findCgroup2Mountpoint()
	if mountpoint == "" {
		logrus.Warn("cgroup2 is not mounted. cgroup2 evacuation is discarded.")
		return nil
	}

	oldGroup := getCgroup2(os.Getpid())
	if mountpoint == "" {
		logrus.Warn("process is not running with cgroup2. cgroup2 evacuation is discarded.")
		return nil
	}

	newGroup := filepath.Join(oldGroup, evac)

	oldPath := filepath.Join(mountpoint, oldGroup)
	newPath := filepath.Join(mountpoint, newGroup)

	if err := os.MkdirAll(newPath, 0755); err != nil {
		return err
	}

	// evacuate existing procs from oldGroup to newGroup, so that we can enable all controllers including threaded ones
	cgroupProcsBytes, err := ioutil.ReadFile(filepath.Join(oldPath, "cgroup.procs"))
	if err != nil {
		return err
	}
	for _, pidStr := range strings.Split(string(cgroupProcsBytes), "\n") {
		if pidStr == "" || pidStr == "0" {
			continue
		}
		if err := ioutil.WriteFile(filepath.Join(newPath, "cgroup.procs"), []byte(pidStr), 0644); err != nil {
			logrus.WithError(err).Warnf("failed to move process %s to cgroup %q", pidStr, newGroup)
		}
	}

	// enable controllers for all subgroups under the oldGroup
	controllerBytes, err := ioutil.ReadFile(filepath.Join(oldPath, "cgroup.controllers"))
	if err != nil {
		return err
	}
	for _, controller := range strings.Fields(string(controllerBytes)) {
		logrus.Debugf("enabling controller %q", controller)
		if err := ioutil.WriteFile(filepath.Join(oldPath, "cgroup.subtree_control"), []byte("+"+controller), 0644); err != nil {
			logrus.WithError(err).Warnf("failed to enable controller %q", controller)
		}
	}

	return nil
}

func findCgroup2Mountpoint() string {
	f := mountinfoFSTypeFilter("cgroup2")
	mounts, err := mountinfo.GetMounts(f)
	if err != nil {
		logrus.WithError(err).Warn("failed to find mountpoint for cgroup2")
		return ""
	}
	if len(mounts) == 0 {
		return ""
	}
	if len(mounts) != 1 {
		logrus.Warnf("expected single mountpoint for cgroup2, got %d", len(mounts))
	}
	return mounts[0].Mountpoint
}

func getCgroup2(pid int) string {
	p := fmt.Sprintf("/proc/%d/cgroup", pid)
	b, err := ioutil.ReadFile(p)
	if err != nil {
		logrus.WithError(err).Warnf("failed to read %q", p)
		return ""
	}
	return getCgroup2FromProcPidCgroup(b)
}

func getCgroup2FromProcPidCgroup(b []byte) string {
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "0::") {
			return strings.TrimPrefix(l, "0::")
		}
	}
	return ""
}
