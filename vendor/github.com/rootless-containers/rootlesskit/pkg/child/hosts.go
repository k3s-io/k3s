package child

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/pkg/errors"
)

// generateEtcHosts makes sure the current hostname is resolved into
// 127.0.0.1 or ::1, not into the host eth0 IP address.
//
// Note that /etc/hosts is not used by nslookup/dig. (Use `getent ahostsv4` instead.)
func generateEtcHosts() ([]byte, error) {
	etcHosts, err := ioutil.ReadFile("/etc/hosts")
	if err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	// FIXME: no need to add the entry if already added
	s := fmt.Sprintf("%s\n127.0.0.1 %s\n::1 %s\n",
		string(etcHosts), hostname, hostname)
	return []byte(s), nil
}

// writeEtcHosts is akin to writeResolvConf
// TODO: dedupe
func writeEtcHosts() error {
	newEtcHosts, err := generateEtcHosts()
	if err != nil {
		return err
	}
	// remove copied-up link
	_ = os.Remove("/etc/hosts")
	if err := ioutil.WriteFile("/etc/hosts", newEtcHosts, 0644); err != nil {
		return errors.Wrapf(err, "writing /etc/hosts")
	}
	return nil
}

// mountEtcHosts is akin to mountResolvConf
// TODO: dedupe
func mountEtcHosts(tempDir string) error {
	newEtcHosts, err := generateEtcHosts()
	if err != nil {
		return err
	}
	myEtcHosts := filepath.Join(tempDir, "hosts")
	if err := ioutil.WriteFile(myEtcHosts, newEtcHosts, 0644); err != nil {
		return errors.Wrapf(err, "writing %s", myEtcHosts)
	}

	if err := unix.Mount(myEtcHosts, "/etc/hosts", "", uintptr(unix.MS_BIND), ""); err != nil {
		return errors.Wrapf(err, "failed to create bind mount /etc/hosts for %s", myEtcHosts)
	}
	return nil
}
