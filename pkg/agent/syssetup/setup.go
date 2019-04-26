package syssetup

import (
	"io/ioutil"
	"os/exec"

	"github.com/sirupsen/logrus"
)

var (
	callIPTablesFile = "/proc/sys/net/bridge/bridge-nf-call-iptables"
	forward          = "/proc/sys/net/ipv4/ip_forward"
)

func Configure() error {
	if err := exec.Command("modprobe", "br_netfilter").Run(); err != nil {
		logrus.Warnf("failed to start br_netfilter module")
		return nil
	}
	if err := ioutil.WriteFile(callIPTablesFile, []byte("1"), 0640); err != nil {
		logrus.Warnf("failed to write value 1 at %s: %v", callIPTablesFile, err)
		return nil
	}
	if err := ioutil.WriteFile(forward, []byte("1"), 0640); err != nil {
		logrus.Warnf("failed to write value 1 at %s: %v", forward, err)
		return nil
	}

	if err := exec.Command("modprobe", "overlay").Run(); err != nil {
		logrus.Warnf("failed to start overlay module")
		return nil
	}
	if err := exec.Command("modprobe", "nf_conntrack").Run(); err != nil {
		logrus.Warnf("failed to start nf_conntrack module")
		return nil
	}
	return nil
}
