// +build linux

package agent

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func createRootlessConfig(argsMap map[string]string, hasCFS, hasPIDs bool) {
	// "/sys/fs/cgroup" is namespaced
	cgroupfsWritable := unix.Access("/sys/fs/cgroup", unix.W_OK) == nil
	if hasCFS && hasPIDs && cgroupfsWritable {
		logrus.Info("cgroup v2 controllers are delegated for rootless.")
		// cgroupfs v2, delegated for rootless by systemd
		argsMap["cgroup-driver"] = "cgroupfs"
	} else {
		logrus.Warn("cgroup v2 controllers are not delegated for rootless. Setting cgroup driver to \"none\".")
		// flags are from https://github.com/rootless-containers/usernetes/blob/v20190826.0/boot/kubelet.sh
		argsMap["cgroup-driver"] = "none"
		argsMap["feature-gates=SupportNoneCgroupDriver"] = "true"
		argsMap["cgroups-per-qos"] = "false"
		argsMap["enforce-node-allocatable"] = ""
	}
}
