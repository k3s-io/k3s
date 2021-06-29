package slirp4netns

import (
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/rootless-containers/rootlesskit/pkg/common"
	"github.com/rootless-containers/rootlesskit/pkg/network"
	"github.com/rootless-containers/rootlesskit/pkg/network/iputils"
	"github.com/rootless-containers/rootlesskit/pkg/network/parentutils"
)

type Features struct {
	// SupportsCIDR --cidr (v0.3.0)
	SupportsCIDR bool
	// SupportsDisableHostLoopback --disable-host-loopback (v0.3.0)
	SupportsDisableHostLoopback bool
	// SupportsAPISocket --api-socket (v0.3.0)
	SupportsAPISocket bool
	// SupportsEnableSandbox --enable-sandbox (v0.4.0)
	SupportsEnableSandbox bool
	// SupportsEnableSeccomp --enable-seccomp (v0.4.0)
	SupportsEnableSeccomp bool
	// KernelSupportsSeccomp whether the kernel supports slirp4netns --enable-seccomp
	KernelSupportsEnableSeccomp bool
}

func DetectFeatures(binary string) (*Features, error) {
	if binary == "" {
		return nil, errors.New("got empty slirp4netns binary")
	}
	realBinary, err := exec.LookPath(binary)
	if err != nil {
		return nil, errors.Wrapf(err, "slirp4netns binary %q is not installed", binary)
	}
	cmd := exec.Command(realBinary, "--help")
	cmd.Env = os.Environ()
	b, err := cmd.CombinedOutput()
	s := string(b)
	if err != nil {
		return nil, errors.Wrapf(err,
			"command \"%s --help\" failed, make sure slirp4netns v0.4.0+ is installed: %q",
			realBinary, s)
	}
	if !strings.Contains(s, "--netns-type") {
		// We don't use --netns-type, but we check the presence of --netns-type to
		// ensure slirp4netns >= v0.4.0: https://github.com/rootless-containers/rootlesskit/issues/143
		return nil, errors.New("slirp4netns seems older than v0.4.0")
	}
	kernelSupportsEnableSeccomp := false
	if unix.Prctl(unix.PR_GET_SECCOMP, 0, 0, 0, 0) != unix.EINVAL {
		kernelSupportsEnableSeccomp = unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, 0, 0, 0) != unix.EINVAL
	}
	f := Features{
		SupportsCIDR:                strings.Contains(s, "--cidr"),
		SupportsDisableHostLoopback: strings.Contains(s, "--disable-host-loopback"),
		SupportsAPISocket:           strings.Contains(s, "--api-socket"),
		SupportsEnableSandbox:       strings.Contains(s, "--enable-sandbox"),
		SupportsEnableSeccomp:       strings.Contains(s, "--enable-seccomp"),
		KernelSupportsEnableSeccomp: kernelSupportsEnableSeccomp,
	}
	return &f, nil
}

// NewParentDriver instantiates new parent driver.
// Requires slirp4netns v0.4.0 or later.
func NewParentDriver(logWriter io.Writer, binary string, mtu int, ipnet *net.IPNet, disableHostLoopback bool, apiSocketPath string, enableSandbox, enableSeccomp bool) (network.ParentDriver, error) {
	if binary == "" {
		return nil, errors.New("got empty slirp4netns binary")
	}
	if mtu < 0 {
		return nil, errors.New("got negative mtu")
	}
	if mtu == 0 {
		mtu = 65520
	}
	features, err := DetectFeatures(binary)
	if err != nil {
		return nil, err
	}
	if ipnet != nil && !features.SupportsCIDR {
		return nil, errors.New("this version of slirp4netns does not support --cidr")
	}
	if disableHostLoopback && !features.SupportsDisableHostLoopback {
		return nil, errors.New("this version of slirp4netns does not support --disable-host-loopback")
	}
	if apiSocketPath != "" && !features.SupportsAPISocket {
		return nil, errors.New("this version of slirp4netns does not support --api-socket")
	}
	if enableSandbox && !features.SupportsEnableSandbox {
		return nil, errors.New("this version of slirp4netns does not support --enable-sandbox")
	}
	if enableSeccomp && !features.SupportsEnableSeccomp {
		return nil, errors.New("this version of slirp4netns does not support --enable-seccomp")
	}
	if enableSeccomp && !features.KernelSupportsEnableSeccomp {
		return nil, errors.New("kernel does not support seccomp")
	}

	return &parentDriver{
		logWriter:           logWriter,
		binary:              binary,
		mtu:                 mtu,
		ipnet:               ipnet,
		disableHostLoopback: disableHostLoopback,
		apiSocketPath:       apiSocketPath,
		enableSandbox:       enableSandbox,
		enableSeccomp:       enableSeccomp,
	}, nil
}

type parentDriver struct {
	logWriter           io.Writer
	binary              string
	mtu                 int
	ipnet               *net.IPNet
	disableHostLoopback bool
	apiSocketPath       string
	enableSandbox       bool
	enableSeccomp       bool
}

func (d *parentDriver) MTU() int {
	return d.mtu
}

func (d *parentDriver) ConfigureNetwork(childPID int, stateDir string) (*common.NetworkMessage, func() error, error) {
	tap := "tap0"
	var cleanups []func() error
	if err := parentutils.PrepareTap(childPID, tap); err != nil {
		return nil, common.Seq(cleanups), errors.Wrapf(err, "setting up tap %s", tap)
	}
	ctx, cancel := context.WithCancel(context.Background())
	readyR, readyW, err := os.Pipe()
	if err != nil {
		return nil, common.Seq(cleanups), err
	}
	defer readyR.Close()
	defer readyW.Close()
	// -r: readyFD (requires slirp4netns >= v0.4.0: https://github.com/rootless-containers/rootlesskit/issues/143)
	opts := []string{"--mtu", strconv.Itoa(d.mtu), "-r", "3"}
	if d.disableHostLoopback {
		opts = append(opts, "--disable-host-loopback")
	}
	if d.ipnet != nil {
		opts = append(opts, "--cidr", d.ipnet.String())
	}
	if d.apiSocketPath != "" {
		opts = append(opts, "--api-socket", d.apiSocketPath)
	}
	if d.enableSandbox {
		opts = append(opts, "--enable-sandbox")
	}
	if d.enableSeccomp {
		opts = append(opts, "--enable-seccomp")
	}
	cmd := exec.CommandContext(ctx, d.binary, append(opts, []string{strconv.Itoa(childPID), tap}...)...)
	// FIXME: Stdout doen't seem captured
	cmd.Stdout = d.logWriter
	cmd.Stderr = d.logWriter
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
	cmd.ExtraFiles = append(cmd.ExtraFiles, readyW)
	cleanups = append(cleanups, func() error {
		logrus.Debugf("killing slirp4netns")
		cancel()
		wErr := cmd.Wait()
		logrus.Debugf("killed slirp4netns: %v", wErr)
		return nil
	})
	if err := cmd.Start(); err != nil {
		return nil, common.Seq(cleanups), errors.Wrapf(err, "executing %v", cmd)
	}

	if err := waitForReadyFD(cmd.Process.Pid, readyR); err != nil {
		return nil, common.Seq(cleanups), errors.Wrapf(err, "waiting for ready fd (%v)", cmd)
	}
	netmsg := common.NetworkMessage{
		Dev: tap,
		MTU: d.mtu,
	}
	if d.ipnet != nil {
		// TODO: get the actual configuration via slirp4netns API?
		x, err := iputils.AddIPInt(d.ipnet.IP, 100)
		if err != nil {
			return nil, common.Seq(cleanups), err
		}
		netmsg.IP = x.String()
		netmsg.Netmask, _ = d.ipnet.Mask.Size()
		x, err = iputils.AddIPInt(d.ipnet.IP, 2)
		if err != nil {
			return nil, common.Seq(cleanups), err
		}
		netmsg.Gateway = x.String()
		x, err = iputils.AddIPInt(d.ipnet.IP, 3)
		if err != nil {
			return nil, common.Seq(cleanups), err
		}
		netmsg.DNS = x.String()
	} else {
		netmsg.IP = "10.0.2.100"
		netmsg.Netmask = 24
		netmsg.Gateway = "10.0.2.2"
		netmsg.DNS = "10.0.2.3"
	}
	return &netmsg, common.Seq(cleanups), nil
}

// waitForReady is from libpod
// https://github.com/containers/libpod/blob/e6b843312b93ddaf99d0ef94a7e60ff66bc0eac8/libpod/networking_linux.go#L272-L308
func waitForReadyFD(cmdPid int, r *os.File) error {
	b := make([]byte, 16)
	for {
		if err := r.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return errors.Wrapf(err, "error setting slirp4netns pipe timeout")
		}
		if _, err := r.Read(b); err == nil {
			break
		} else {
			if os.IsTimeout(err) {
				// Check if the process is still running.
				var status syscall.WaitStatus
				pid, err := syscall.Wait4(cmdPid, &status, syscall.WNOHANG, nil)
				if err != nil {
					return errors.Wrapf(err, "failed to read slirp4netns process status")
				}
				if pid != cmdPid {
					continue
				}
				if status.Exited() {
					return errors.New("slirp4netns failed")
				}
				if status.Signaled() {
					return errors.New("slirp4netns killed by signal")
				}
				continue
			}
			return errors.Wrapf(err, "failed to read from slirp4netns sync pipe")
		}
	}
	return nil
}

func NewChildDriver() network.ChildDriver {
	return &childDriver{}
}

type childDriver struct {
}

func (d *childDriver) ConfigureNetworkChild(netmsg *common.NetworkMessage) (string, error) {
	tap := netmsg.Dev
	if tap == "" {
		return "", errors.New("could not determine the preconfigured tap")
	}
	// tap is created and "up".
	// IP stuff and MTU are not configured by the parent here,
	// and they are up to the child.
	return tap, nil
}
