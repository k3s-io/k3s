package child

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/rootless-containers/rootlesskit/pkg/common"
	"github.com/rootless-containers/rootlesskit/pkg/copyup"
	"github.com/rootless-containers/rootlesskit/pkg/msgutil"
	"github.com/rootless-containers/rootlesskit/pkg/network"
	"github.com/rootless-containers/rootlesskit/pkg/port"
	"github.com/rootless-containers/rootlesskit/pkg/sigproxy"
	sigproxysignal "github.com/rootless-containers/rootlesskit/pkg/sigproxy/signal"
)

var propagationStates = map[string]uintptr{
	"private":  uintptr(unix.MS_PRIVATE),
	"rprivate": uintptr(unix.MS_REC | unix.MS_PRIVATE),
	"shared":   uintptr(unix.MS_SHARED),
	"rshared":  uintptr(unix.MS_REC | unix.MS_SHARED),
	"slave":    uintptr(unix.MS_SLAVE),
	"rslave":   uintptr(unix.MS_REC | unix.MS_SLAVE),
}

func createCmd(targetCmd []string) (*exec.Cmd, error) {
	var args []string
	if len(targetCmd) > 1 {
		args = targetCmd[1:]
	}
	cmd := exec.Command(targetCmd[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
	return cmd, nil
}

// mountSysfs is needed for mounting /sys/class/net
// when netns is unshared.
func mountSysfs() error {
	tmp, err := ioutil.TempDir("/tmp", "rksys")
	if err != nil {
		return errors.Wrap(err, "creating a directory under /tmp")
	}
	defer os.RemoveAll(tmp)
	cgroupDir := "/sys/fs/cgroup"
	if err := unix.Mount(cgroupDir, tmp, "", uintptr(unix.MS_BIND|unix.MS_REC), ""); err != nil {
		return errors.Wrapf(err, "failed to create bind mount on %s", cgroupDir)
	}

	if err := unix.Mount("none", "/sys", "sysfs", 0, ""); err != nil {
		// when the sysfs in the parent namespace is RO,
		// we can't mount RW sysfs even in the child namespace.
		// https://github.com/rootless-containers/rootlesskit/pull/23#issuecomment-429292632
		// https://github.com/torvalds/linux/blob/9f203e2f2f065cd74553e6474f0ae3675f39fb0f/fs/namespace.c#L3326-L3328
		logrus.Warnf("failed to mount sysfs, falling back to read-only mount: %v", err)
		if err := unix.Mount("none", "/sys", "sysfs", uintptr(unix.MS_RDONLY), ""); err != nil {
			// when /sys/firmware is masked, even RO sysfs can't be mounted
			logrus.Warnf("failed to mount sysfs: %v", err)
		}
	}
	if err := unix.Mount(tmp, cgroupDir, "", uintptr(unix.MS_MOVE), ""); err != nil {
		return errors.Wrapf(err, "failed to move mount point from %s to %s", tmp, cgroupDir)
	}
	return nil
}

func mountProcfs() error {
	if err := unix.Mount("none", "/proc", "proc", 0, ""); err != nil {
		logrus.Warnf("failed to mount procfs, falling back to read-only mount: %v", err)
		if err := unix.Mount("none", "/proc", "proc", uintptr(unix.MS_RDONLY), ""); err != nil {
			logrus.Warnf("failed to mount procfs: %v", err)
		}
	}
	return nil
}

func activateLoopback() error {
	cmds := [][]string{
		{"ip", "link", "set", "lo", "up"},
	}
	if err := common.Execs(os.Stderr, os.Environ(), cmds); err != nil {
		return errors.Wrapf(err, "executing %v", cmds)
	}
	return nil
}

func activateDev(dev, ip string, netmask int, gateway string, mtu int) error {
	cmds := [][]string{
		{"ip", "link", "set", dev, "up"},
		{"ip", "link", "set", "dev", dev, "mtu", strconv.Itoa(mtu)},
		{"ip", "addr", "add", ip + "/" + strconv.Itoa(netmask), "dev", dev},
		{"ip", "route", "add", "default", "via", gateway, "dev", dev},
	}
	if err := common.Execs(os.Stderr, os.Environ(), cmds); err != nil {
		return errors.Wrapf(err, "executing %v", cmds)
	}
	return nil
}

func setupCopyDir(driver copyup.ChildDriver, dirs []string) (bool, error) {
	if driver != nil {
		etcWasCopied := false
		copied, err := driver.CopyUp(dirs)
		for _, d := range copied {
			if d == "/etc" {
				etcWasCopied = true
				break
			}
		}
		return etcWasCopied, err
	}
	if len(dirs) != 0 {
		return false, errors.New("copy-up driver is not specified")
	}
	return false, nil
}

func setupNet(msg common.Message, etcWasCopied bool, driver network.ChildDriver) error {
	// HostNetwork
	if driver == nil {
		return nil
	}
	// for /sys/class/net
	if err := mountSysfs(); err != nil {
		return err
	}
	if err := activateLoopback(); err != nil {
		return err
	}
	dev, err := driver.ConfigureNetworkChild(&msg.Network)
	if err != nil {
		return err
	}
	if err := activateDev(dev, msg.Network.IP, msg.Network.Netmask, msg.Network.Gateway, msg.Network.MTU); err != nil {
		return err
	}
	if etcWasCopied {
		if err := writeResolvConf(msg.Network.DNS); err != nil {
			return err
		}
		if err := writeEtcHosts(); err != nil {
			return err
		}
	} else {
		logrus.Warn("Mounting /etc/resolv.conf without copying-up /etc. " +
			"Note that /etc/resolv.conf in the namespace will be unmounted when it is recreated on the host. " +
			"Unless /etc/resolv.conf is statically configured, copying-up /etc is highly recommended. " +
			"Please refer to RootlessKit documentation for further information.")
		if err := mountResolvConf(msg.StateDir, msg.Network.DNS); err != nil {
			return err
		}
		if err := mountEtcHosts(msg.StateDir); err != nil {
			return err
		}
	}
	return nil
}

type Opt struct {
	PipeFDEnvKey  string              // needs to be set
	TargetCmd     []string            // needs to be set
	NetworkDriver network.ChildDriver // nil for HostNetwork
	CopyUpDriver  copyup.ChildDriver  // cannot be nil if len(CopyUpDirs) != 0
	CopyUpDirs    []string
	PortDriver    port.ChildDriver
	MountProcfs   bool   // needs to be set if (and only if) parent.Opt.CreatePIDNS is set
	Propagation   string // mount propagation type
	Reaper        bool
}

func Child(opt Opt) error {
	if opt.PipeFDEnvKey == "" {
		return errors.New("pipe FD env key is not set")
	}
	pipeFDStr := os.Getenv(opt.PipeFDEnvKey)
	if pipeFDStr == "" {
		return errors.Errorf("%s is not set", opt.PipeFDEnvKey)
	}
	pipeFD, err := strconv.Atoi(pipeFDStr)
	if err != nil {
		return errors.Wrapf(err, "unexpected fd value: %s", pipeFDStr)
	}
	pipeR := os.NewFile(uintptr(pipeFD), "")
	var msg common.Message
	if _, err := msgutil.UnmarshalFromReader(pipeR, &msg); err != nil {
		return errors.Wrapf(err, "parsing message from fd %d", pipeFD)
	}
	logrus.Debugf("child: got msg from parent: %+v", msg)
	if msg.Stage == 0 {
		// the parent has configured the child's uid_map and gid_map, but the child doesn't have caps here.
		// so we exec the child again to obtain caps.
		// PID should be kept.
		if err = syscall.Exec("/proc/self/exe", os.Args, os.Environ()); err != nil {
			return err
		}
		panic("should not reach here")
	}
	if msg.Stage != 1 {
		return errors.Errorf("expected stage 1, got stage %d", msg.Stage)
	}
	// The parent calls child with Pdeathsig, but it is cleared when newuidmap SUID binary is called
	// https://github.com/rootless-containers/rootlesskit/issues/65#issuecomment-492343646
	runtime.LockOSThread()
	err = unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(unix.SIGKILL), 0, 0, 0)
	runtime.UnlockOSThread()
	if err != nil {
		return err
	}
	os.Unsetenv(opt.PipeFDEnvKey)
	if err := pipeR.Close(); err != nil {
		return errors.Wrapf(err, "failed to close fd %d", pipeFD)
	}
	if msg.StateDir == "" {
		return errors.New("got empty StateDir")
	}
	if err := setMountPropagation(opt.Propagation); err != nil {
		return err
	}
	etcWasCopied, err := setupCopyDir(opt.CopyUpDriver, opt.CopyUpDirs)
	if err != nil {
		return err
	}
	if err := setupNet(msg, etcWasCopied, opt.NetworkDriver); err != nil {
		return err
	}
	if opt.MountProcfs {
		if err := mountProcfs(); err != nil {
			return err
		}
	}
	portQuitCh := make(chan struct{})
	portErrCh := make(chan error)
	if opt.PortDriver != nil {
		go func() {
			portErrCh <- opt.PortDriver.RunChildDriver(msg.Port.Opaque, portQuitCh)
		}()
	}

	cmd, err := createCmd(opt.TargetCmd)
	if err != nil {
		return err
	}
	if opt.Reaper {
		if err := runAndReap(cmd); err != nil {
			return errors.Wrapf(err, "command %v exited", opt.TargetCmd)
		}
	} else {
		if err := cmd.Start(); err != nil {
			return errors.Wrapf(err, "command %v exited", opt.TargetCmd)
		}
		sigc := sigproxy.ForwardAllSignals(context.TODO(), cmd.Process.Pid)
		defer sigproxysignal.StopCatch(sigc)
		if err := cmd.Wait(); err != nil {
			return errors.Wrapf(err, "command %v exited", opt.TargetCmd)
		}
	}
	if opt.PortDriver != nil {
		portQuitCh <- struct{}{}
		return <-portErrCh
	}
	return nil
}

func setMountPropagation(propagation string) error {
	flags, ok := propagationStates[propagation]
	if ok {
		if err := unix.Mount("none", "/", "", flags, ""); err != nil {
			return errors.Wrapf(err, "failed to share mount point: /")
		}
	}
	return nil
}

func runAndReap(cmd *exec.Cmd) error {
	c := make(chan os.Signal, 32)
	signal.Notify(c, syscall.SIGCHLD)
	if err := cmd.Start(); err != nil {
		return err
	}
	sigc := sigproxy.ForwardAllSignals(context.TODO(), cmd.Process.Pid)
	defer sigproxysignal.StopCatch(sigc)

	result := make(chan error)
	go func() {
		defer close(result)
		for range c {
			for {
				if pid, err := syscall.Wait4(-1, nil, syscall.WNOHANG, nil); err != nil || pid <= 0 {
					break
				} else {
					if pid == cmd.Process.Pid {
						result <- cmd.Wait()
					}
				}
			}
		}
	}()
	return <-result
}
