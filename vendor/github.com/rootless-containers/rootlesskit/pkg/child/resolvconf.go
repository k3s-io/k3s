package child

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/rootless-containers/rootlesskit/pkg/common"
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
	cmds := [][]string{
		{"mount", "--bind", myResolvConf, "/etc/resolv.conf"},
	}
	if err := common.Execs(os.Stderr, os.Environ(), cmds); err != nil {
		return errors.Wrapf(err, "executing %v", cmds)
	}
	return nil
}
