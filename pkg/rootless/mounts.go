// +build !windows

package rootless

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func setupMounts(stateDir string) error {
	mountMap := [][]string{
		{"/run", ""},
		{"/var/run", ""},
		{"/var/log", filepath.Join(stateDir, "logs")},
		{"/var/lib/cni", filepath.Join(stateDir, "cni")},
		{"/var/lib/kubelet", filepath.Join(stateDir, "kubelet")},
		{"/etc/rancher", filepath.Join(stateDir, "etc", "rancher")},
	}

	for _, v := range mountMap {
		if err := setupMount(v[0], v[1]); err != nil {
			return errors.Wrapf(err, "failed to setup mount %s => %s", v[0], v[1])
		}
	}

	return nil
}

func setupMount(target, dir string) error {
	toCreate := target
	for {
		if toCreate == "/" {
			return fmt.Errorf("missing /%s on the root filesystem", strings.Split(target, "/")[0])
		}

		if err := os.MkdirAll(toCreate, 0700); err == nil {
			break
		}

		toCreate = filepath.Base(toCreate)
	}

	if err := os.MkdirAll(toCreate, 0700); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", toCreate)
	}

	logrus.Debug("Mounting none ", toCreate, " tmpfs")
	if err := unix.Mount("none", toCreate, "tmpfs", 0, ""); err != nil {
		return errors.Wrapf(err, "failed to mount tmpfs to %s", toCreate)
	}

	if err := os.MkdirAll(target, 0700); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", target)
	}

	if dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dir)
	}

	logrus.Debug("Mounting ", dir, target, " none bind")
	return unix.Mount(dir, target, "none", unix.MS_BIND, "")
}
