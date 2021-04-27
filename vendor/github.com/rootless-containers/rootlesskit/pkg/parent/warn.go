package parent

import (
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/moby/sys/mountinfo"
	"github.com/sirupsen/logrus"
)

func warnPropagation(propagation string) {
	mounts, err := mountinfo.GetMounts(mountinfo.SingleEntryFilter("/"))
	if err != nil || len(mounts) < 1 {
		logrus.WithError(err).Warn("Failed to parse mountinfo")
		return
	}
	root := mounts[0]
	// 1. When running on a "sane" host,   root.Optional is like "shared:1".   ("shared" in findmnt(8) output)
	// 2. When running inside a container, root.Optional is like "master:363". ("private, slave" in findmnt(8) output)
	//
	// Setting non-private propagation is supported for 1, unsupported for 2.
	if !strings.Contains(propagation, "private") && !strings.Contains(root.Optional, "shared") {
		logrus.Warnf("The host root filesystem is mounted as %q. Setting child propagation to %q is not supported.",
			root.Optional, propagation)
	}
}

// warnSysctl verifies /proc/sys/kernel/unprivileged_userns_clone and /proc/sys/user/max_user_namespaces
func warnSysctl() {
	uuc, err := ioutil.ReadFile("/proc/sys/kernel/unprivileged_userns_clone")
	// The file exists only on distros with the "add sysctl to disallow unprivileged CLONE_NEWUSER by default" patch.
	// (e.g. Debian and Arch)
	if err == nil {
		s := strings.TrimSpace(string(uuc))
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			logrus.WithError(err).Warnf("Failed to parse /proc/sys/kernel/unprivileged_userns_clone (%q)", s)
		} else if i == 0 {
			logrus.Warn("/proc/sys/kernel/unprivileged_userns_clone needs to be set to 1.")
		}
	}

	mun, err := ioutil.ReadFile("/proc/sys/user/max_user_namespaces")
	if err == nil {
		s := strings.TrimSpace(string(mun))
		i, err := strconv.ParseInt(strings.TrimSpace(string(mun)), 10, 64)
		if err != nil {
			logrus.WithError(err).Warnf("Failed to parse /proc/sys/user/max_user_namespaces (%q)", s)
		} else if i == 0 {
			logrus.Warn("/proc/sys/user/max_user_namespaces needs to be set to non-zero.")
		} else {
			threshold := int64(1024)
			if i < threshold {
				logrus.Warnf("/proc/sys/user/max_user_namespaces=%d may be low. Consider setting to >= %d.", i, threshold)
			}
		}
	}
}
