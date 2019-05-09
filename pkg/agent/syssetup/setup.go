package syssetup

import (
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
)

var (
	callIPTablesFile = "/proc/sys/net/bridge/bridge-nf-call-iptables"
	forward          = "/proc/sys/net/ipv4/ip_forward"
)

func loadKernelModule(moduleName string) {
	if _, err := os.Stat("/sys/module/" + moduleName); err == nil {
		logrus.Infof("module %s was already loaded", moduleName)
		return
	}

	if err := exec.Command("modprobe", moduleName).Run(); err != nil {
		logrus.Warnf("failed to start %s module", moduleName)
	}
}

func Configure() error {
	loadKernelModule("br_netfilter")

	if err := ioutil.WriteFile(callIPTablesFile, []byte("1"), 0640); err != nil {
		logrus.Warnf("failed to write value 1 at %s: %v", callIPTablesFile, err)
		return nil
	}
	if err := ioutil.WriteFile(forward, []byte("1"), 0640); err != nil {
		logrus.Warnf("failed to write value 1 at %s: %v", forward, err)
		return nil
	}

	loadKernelModule("overlay")
	loadKernelModule("nf_conntrack")

	return nil
}
