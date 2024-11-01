//go:build !windows
// +build !windows

package rootless

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/pkg/errors"
	"github.com/rootless-containers/rootlesskit/pkg/child"
	"github.com/rootless-containers/rootlesskit/pkg/copyup/tmpfssymlink"
	"github.com/rootless-containers/rootlesskit/pkg/network/slirp4netns"
	"github.com/rootless-containers/rootlesskit/pkg/parent"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	pipeFD             = "_K3S_ROOTLESS_FD"
	childEnv           = "_K3S_ROOTLESS_SOCK"
	evacuateCgroup2Env = "_K3S_ROOTLESS_EVACUATE_CGROUP2" // boolean
	Sock               = ""

	mtuEnv             = "K3S_ROOTLESS_MTU"
	cidrEnv            = "K3S_ROOTLESS_CIDR"
	enableIPv6Env      = "K3S_ROOTLESS_ENABLE_IPV6"
	portDriverEnv      = "K3S_ROOTLESS_PORT_DRIVER"
	disableLoopbackEnv = "K3S_ROOTLESS_DISABLE_HOST_LOOPBACK"
	copyUpDirsEnv      = "K3S_ROOTLESS_COPYUPDIRS"
)

func Rootless(stateDir string, enableIPv6 bool) error {
	defer func() {
		os.Unsetenv(pipeFD)
		os.Unsetenv(childEnv)
	}()

	hasFD := os.Getenv(pipeFD) != ""
	hasChildEnv := os.Getenv(childEnv) != ""
	rootlessDir := filepath.Join(stateDir, "rootless")
	driver := getDriver(strings.ToLower(os.Getenv(portDriverEnv)))

	if hasFD {
		logrus.Debug("Running rootless child")
		childOpt, err := createChildOpt(driver)
		if err != nil {
			logrus.Fatal(err)
		}
		if err := child.Child(*childOpt); err != nil {
			logrus.Fatalf("child died: %v", err)
		}
	}

	if hasChildEnv {
		Sock = os.Getenv(childEnv)
		logrus.Debug("Running rootless process")
		return setupMounts(stateDir)
	}

	logrus.Debug("Running rootless parent")
	if err := validateSysctl(); err != nil {
		logrus.Fatal(err)
	}
	parentOpt, err := createParentOpt(driver, rootlessDir, enableIPv6)
	if err != nil {
		logrus.Fatal(err)
	}

	os.Setenv(childEnv, filepath.Join(parentOpt.StateDir, parent.StateFileAPISock))
	if parentOpt.EvacuateCgroup2 != "" {
		os.Setenv(evacuateCgroup2Env, "1")
	}
	if err := parent.Parent(*parentOpt); err != nil {
		logrus.Fatal(err)
	}
	os.Exit(0)

	return nil
}

func validateSysctl() error {
	expected := map[string]string{
		// kernel.unprivileged_userns_clone needs to be 1 to allow userns on some distros.
		"kernel.unprivileged_userns_clone": "1",

		// net.ipv4.ip_forward should not need to be 1 in the parent namespace.
		// However, the current k3s implementation has a bug that requires net.ipv4.ip_forward=1
		// https://github.com/k3s-io/k3s/issues/2420#issuecomment-715051120
		"net.ipv4.ip_forward": "1",
	}
	for key, expectedValue := range expected {
		if actualValue, err := readSysctl(key); err == nil {
			if expectedValue != actualValue {
				return errors.Errorf("expected sysctl value %q to be %q, got %q; try adding \"%s=%s\" to /etc/sysctl.conf and running `sudo sysctl --system`",
					key, expectedValue, actualValue, key, expectedValue)
			}
		}
	}
	return nil
}

func readSysctl(key string) (string, error) {
	p := "/proc/sys/" + strings.ReplaceAll(key, ".", "/")
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func parseCIDR(s string) (*net.IPNet, error) {
	if s == "" {
		return nil, nil
	}
	ip, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}
	if !ip.Equal(ipnet.IP) {
		return nil, errors.Errorf("cidr must be like 10.0.2.0/24, not like 10.0.2.100/24")
	}
	return ipnet, nil
}

func createParentOpt(driver portDriver, stateDir string, enableIPv6 bool) (*parent.Opt, error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to mkdir %s", stateDir)
	}

	driver.SetStateDir(stateDir)

	opt := &parent.Opt{
		StateDir:       stateDir,
		CreatePIDNS:    true,
		CreateCgroupNS: true,
		CreateUTSNS:    true,
		CreateIPCNS:    true,
	}

	selfCgroupMap, err := cgroups.ParseCgroupFile("/proc/self/cgroup")
	if err != nil {
		return nil, err
	}
	if selfCgroup2 := selfCgroupMap[""]; selfCgroup2 == "" {
		logrus.Warnf("Enabling cgroup2 is highly recommended, see https://rootlesscontaine.rs/getting-started/common/cgroup2/")
	} else {
		selfCgroup2Dir := filepath.Join("/sys/fs/cgroup", selfCgroup2)
		if unix.Access(selfCgroup2Dir, unix.W_OK) == nil {
			opt.EvacuateCgroup2 = "k3s_evac"
		} else {
			logrus.Warn("Cannot set cgroup2 evacuation, make sure to run k3s as a systemd unit")
		}
	}

	mtu := 0
	if val := os.Getenv(mtuEnv); val != "" {
		if v, err := strconv.ParseInt(val, 10, 0); err != nil {
			logrus.Warn("Failed to parse rootless mtu value; using default")
		} else {
			mtu = int(v)
		}
	}

	disableHostLoopback := true
	if val := os.Getenv(disableLoopbackEnv); val != "" {
		if v, err := strconv.ParseBool(val); err != nil {
			logrus.Warn("Failed to parse rootless disable-host-loopback value; using default")
		} else {
			disableHostLoopback = v
		}
	}

	if val := os.Getenv(enableIPv6Env); val != "" {
		if v, err := strconv.ParseBool(val); err != nil {
			logrus.Warn("Failed to parse rootless enable-ipv6 value; using default")
		} else {
			enableIPv6 = v
		}
	}

	cidr := "10.41.0.0/16"
	if val := os.Getenv(cidrEnv); val != "" {
		cidr = val
	}

	ipnet, err := parseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	binary := "slirp4netns"
	if _, err := exec.LookPath(binary); err != nil {
		return nil, err
	}
	opt.NetworkDriver, err = slirp4netns.NewParentDriver(driver.LogWriter(), binary, mtu, ipnet, "tap0", disableHostLoopback, driver.APISocketPath(), false, false, enableIPv6)
	if err != nil {
		return nil, err
	}

	opt.PortDriver, err = driver.NewParentDriver()
	if err != nil {
		return nil, err
	}

	opt.PipeFDEnvKey = pipeFD

	return opt, nil
}

func createChildOpt(driver portDriver) (*child.Opt, error) {
	opt := &child.Opt{}
	opt.TargetCmd = os.Args
	opt.PipeFDEnvKey = pipeFD
	opt.NetworkDriver = slirp4netns.NewChildDriver()
	opt.PortDriver = driver.NewChildDriver()
	opt.CopyUpDirs = []string{"/etc", "/var/run", "/run", "/var/lib"}
	if copyUpDirs := os.Getenv(copyUpDirsEnv); copyUpDirs != "" {
		opt.CopyUpDirs = append(opt.CopyUpDirs, strings.Split(copyUpDirs, ",")...)
	}
	opt.CopyUpDriver = tmpfssymlink.NewChildDriver()
	opt.MountProcfs = true
	opt.Reaper = true
	if v := os.Getenv(evacuateCgroup2Env); v != "" {
		var err error
		opt.EvacuateCgroup2, err = strconv.ParseBool(v)
		if err != nil {
			return nil, err
		}
	}
	return opt, nil
}
