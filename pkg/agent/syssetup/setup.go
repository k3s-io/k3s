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
	exec.Command("modprobe", "br_netfilter").Run()
	if err := ioutil.WriteFile(callIPTablesFile, []byte("1"), 0640); err != nil {
		logrus.Warnf("failed to write value 1 at %s: %v", callIPTablesFile, err)
	}
	if err := ioutil.WriteFile(forward, []byte("1"), 0640); err != nil {
		logrus.Warnf("failed to write value 1 at %s: %v", forward, err)
	}
	return nil
}
