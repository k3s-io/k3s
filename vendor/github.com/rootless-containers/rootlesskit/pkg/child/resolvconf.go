package child

import (
	"golang.org/x/sys/unix"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func generateResolvConf(dns string) []byte {
	return []byte("nameserver " + dns + "\n")
}

func writeResolvConf(dns string) error {
	// remove copied-up link
	_ = os.Remove("/etc/resolv.conf")
	if err := ioutil.WriteFile("/etc/resolv.conf", generateResolvConf(dns), 0644); err != nil {
		return errors.Wrapf(err, "writing %s", "/etc/resolv.conf")
	}
	return nil
}

// mountResolvConf does not work when /etc/resolv.conf is a managed by
// systemd or NetworkManager, because our bind-mounted /etc/resolv.conf (in our namespaces)
// is unexpectedly unmounted when /etc/resolv.conf is recreated in the initial initial namespace.
//
// If /etc/resolv.conf is a symlink, e.g. to ../run/systemd/resolve/stub-resolv.conf,
// our bind-mounted /etc/resolv.conf is still unmounted when /run/systemd/resolve/stub-resolv.conf is recreated.
//
// Use writeResolvConf with copying-up /etc for most cases.
func mountResolvConf(tempDir, dns string) error {
	myResolvConf := filepath.Join(tempDir, "resolv.conf")
	if err := ioutil.WriteFile(myResolvConf, generateResolvConf(dns), 0644); err != nil {
		return errors.Wrapf(err, "writing %s", myResolvConf)
	}

	if err := unix.Mount(myResolvConf, "/etc/resolv.conf", "", uintptr(unix.MS_BIND), ""); err != nil {
		return errors.Wrapf(err, "failed to create bind mount /etc/resolv.conf for %s", myResolvConf)
	}
	return nil
}
