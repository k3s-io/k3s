package parent

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/docker/docker/pkg/idtools"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/theckman/go-flock"

	"github.com/rootless-containers/rootlesskit/pkg/api/router"
	"github.com/rootless-containers/rootlesskit/pkg/common"
	"github.com/rootless-containers/rootlesskit/pkg/msgutil"
	"github.com/rootless-containers/rootlesskit/pkg/network"
	"github.com/rootless-containers/rootlesskit/pkg/port"
)

type Opt struct {
	PipeFDEnvKey   string               // needs to be set
	StateDir       string               // directory needs to be precreated
	StateDirEnvKey string               // optional env key to propagate StateDir value
	NetworkDriver  network.ParentDriver // nil for HostNetwork
	PortDriver     port.ParentDriver    // nil for --port-driver=none
}

// Documented state files. Undocumented ones are subject to change.
const (
	StateFileLock     = "lock"
	StateFileChildPID = "child_pid" // decimal pid number text
	StateFileAPISock  = "api.sock"  // REST API Socket
)

func Parent(opt Opt) error {
	if opt.PipeFDEnvKey == "" {
		return errors.New("pipe FD env key is not set")
	}
	if opt.StateDir == "" {
		return errors.New("state dir is not set")
	}
	if !filepath.IsAbs(opt.StateDir) {
		return errors.New("state dir must be absolute")
	}
	if stat, err := os.Stat(opt.StateDir); err != nil || !stat.IsDir() {
		return errors.Wrap(err, "state dir is inaccessible")
	}
	lockPath := filepath.Join(opt.StateDir, StateFileLock)
	lock := flock.NewFlock(lockPath)
	locked, err := lock.TryLock()
	if err != nil {
		return errors.Wrapf(err, "failed to lock %s", lockPath)
	}
	if !locked {
		return errors.Errorf("failed to lock %s, another RootlessKit is running with the same state directory?", lockPath)
	}
	defer os.RemoveAll(opt.StateDir)
	defer lock.Unlock()
	// when the previous execution crashed, the state dir may not be removed successfully.
	// explicitly remove everything in the state dir except the lock file here.
	for _, f := range []string{StateFileChildPID} {
		p := filepath.Join(opt.StateDir, f)
		if err := os.RemoveAll(p); err != nil {
			return errors.Wrapf(err, "failed to remove %s", p)
		}
	}

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd := exec.Command("/proc/self/exe", os.Args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig:    syscall.SIGKILL,
		Cloneflags:   syscall.CLONE_NEWUSER,
		Unshareflags: syscall.CLONE_NEWNS,
	}
	if opt.NetworkDriver != nil {
		cmd.SysProcAttr.Unshareflags |= syscall.CLONE_NEWNET
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = []*os.File{pipeR}
	cmd.Env = append(os.Environ(), opt.PipeFDEnvKey+"=3")
	if opt.StateDirEnvKey != "" {
		cmd.Env = append(cmd.Env, opt.StateDirEnvKey+"="+opt.StateDir)
	}
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start the child")
	}
	childPIDPath := filepath.Join(opt.StateDir, StateFileChildPID)
	if err := ioutil.WriteFile(childPIDPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0444); err != nil {
		return errors.Wrapf(err, "failed to write the child PID %d to %s", cmd.Process.Pid, childPIDPath)
	}
	if err := setupUIDGIDMap(cmd.Process.Pid); err != nil {
		return errors.Wrap(err, "failed to setup UID/GID map")
	}
	// send message 0
	msg := common.Message{
		Stage:    0,
		Message0: common.Message0{},
	}
	if _, err := msgutil.MarshalToWriter(pipeW, &msg); err != nil {
		return err
	}

	// configure Network driver
	msg = common.Message{
		Stage: 1,
		Message1: common.Message1{
			StateDir: opt.StateDir,
		},
	}
	if opt.NetworkDriver != nil {
		netMsg, cleanupNetwork, err := opt.NetworkDriver.ConfigureNetwork(cmd.Process.Pid, opt.StateDir)
		if cleanupNetwork != nil {
			defer cleanupNetwork()
		}
		if err != nil {
			return errors.Wrapf(err, "failed to setup network %+v", opt.NetworkDriver)
		}
		msg.Message1.Network = *netMsg
	}

	// configure Port driver
	portDriverInitComplete := make(chan struct{})
	portDriverQuit := make(chan struct{})
	portDriverErr := make(chan error)
	if opt.PortDriver != nil {
		msg.Message1.Port.Opaque = opt.PortDriver.OpaqueForChild()
		cctx := &port.ChildContext{
			PID: cmd.Process.Pid,
			IP:  net.ParseIP(msg.Network.IP).To4(),
		}
		go func() {
			portDriverErr <- opt.PortDriver.RunParentDriver(portDriverInitComplete,
				portDriverQuit, cctx)
		}()
	}

	// send message 1
	if _, err := msgutil.MarshalToWriter(pipeW, &msg); err != nil {
		return err
	}
	if err := pipeW.Close(); err != nil {
		return err
	}
	// wait for port driver to be ready
	if opt.PortDriver != nil {
		select {
		case <-portDriverInitComplete:
		case err = <-portDriverErr:
			return err
		}
	}
	// listens the API
	apiSockPath := filepath.Join(opt.StateDir, StateFileAPISock)
	apiCloser, err := listenServeAPI(apiSockPath, &router.Backend{PortDriver: opt.PortDriver})
	if err != nil {
		return err
	}
	// block until the child exits
	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "child exited")
	}
	// close the API socket
	if err := apiCloser.Close(); err != nil {
		return errors.Wrapf(err, "failed to close %s", apiSockPath)
	}
	// shut down port driver
	if opt.PortDriver != nil {
		portDriverQuit <- struct{}{}
		err = <-portDriverErr
	}
	return err
}

func newugidmapArgs() ([]string, []string, error) {
	u, err := user.Current()
	if err != nil {
		return nil, nil, err
	}
	uidMap := []string{
		"0",
		u.Uid,
		"1",
	}
	gidMap := []string{
		"0",
		u.Gid,
		"1",
	}

	// get both subid maps
	// uses username for groupname in case primary groupname is not the same
	// idtools will fall back to getent if /etc/passwd does not contain username
	// works with external auth, ie sssd, ldap, nis
	ims, err := idtools.NewIdentityMapping(u.Username, u.Username)
	if err != nil {
		return nil, nil, err
	}

	uidMapLast := 1
	for _, im := range ims.UIDs() {
		uidMap = append(uidMap, []string{
			strconv.Itoa(uidMapLast),
			strconv.Itoa(im.HostID),
			strconv.Itoa(im.Size),
		}...)
		uidMapLast += im.Size
	}
	gidMapLast := 1
	for _, im := range ims.GIDs() {
		gidMap = append(gidMap, []string{
			strconv.Itoa(gidMapLast),
			strconv.Itoa(im.HostID),
			strconv.Itoa(im.Size),
		}...)
		gidMapLast += im.Size
	}

	return uidMap, gidMap, nil
}

func setupUIDGIDMap(pid int) error {
	uArgs, gArgs, err := newugidmapArgs()
	if err != nil {
		return errors.Wrap(err, "failed to compute uid/gid map")
	}
	pidS := strconv.Itoa(pid)
	cmd := exec.Command("newuidmap", append([]string{pidS}, uArgs...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "newuidmap %s %v failed: %s", pidS, uArgs, string(out))
	}
	cmd = exec.Command("newgidmap", append([]string{pidS}, gArgs...)...)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "newgidmap %s %v failed: %s", pidS, gArgs, string(out))
	}
	return nil
}

// apiCloser is implemented by *http.Server
type apiCloser interface {
	Close() error
	Shutdown(context.Context) error
}

func listenServeAPI(socketPath string, backend *router.Backend) (apiCloser, error) {
	r := mux.NewRouter()
	router.AddRoutes(r, backend)
	srv := &http.Server{Handler: r}
	err := os.RemoveAll(socketPath)
	if err != nil {
		return nil, err
	}
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	go srv.Serve(l)
	return srv, nil
}
