// +build linux

package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"

	"github.com/containerd/console"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/pkg/errors"
)

type tty struct {
	epoller     *console.Epoller
	console     *console.EpollConsole
	hostConsole console.Console
	closers     []io.Closer
	postStart   []io.Closer
	wg          sync.WaitGroup
	consoleC    chan error
}

func (t *tty) copyIO(w io.Writer, r io.ReadCloser) {
	defer t.wg.Done()
	io.Copy(w, r)
	r.Close()
}

// setup pipes for the process so that advanced features like c/r are able to easily checkpoint
// and restore the process's IO without depending on a host specific path or device
func setupProcessPipes(p *libcontainer.Process, rootuid, rootgid int) (*tty, error) {
	i, err := p.InitializeIO(rootuid, rootgid)
	if err != nil {
		return nil, err
	}
	t := &tty{
		closers: []io.Closer{
			i.Stdin,
			i.Stdout,
			i.Stderr,
		},
	}
	// add the process's io to the post start closers if they support close
	for _, cc := range []interface{}{
		p.Stdin,
		p.Stdout,
		p.Stderr,
	} {
		if c, ok := cc.(io.Closer); ok {
			t.postStart = append(t.postStart, c)
		}
	}
	go func() {
		io.Copy(i.Stdin, os.Stdin)
		i.Stdin.Close()
	}()
	t.wg.Add(2)
	go t.copyIO(os.Stdout, i.Stdout)
	go t.copyIO(os.Stderr, i.Stderr)
	return t, nil
}

func inheritStdio(process *libcontainer.Process) error {
	process.Stdin = os.Stdin
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	return nil
}

func (t *tty) initHostConsole() error {
	// Usually all three (stdin, stdout, and stderr) streams are open to
	// the terminal, but they might be redirected, so try them all.
	for _, s := range []*os.File{os.Stderr, os.Stdout, os.Stdin} {
		c, err := console.ConsoleFromFile(s)
		switch err {
		case nil:
			t.hostConsole = c
			return nil
		case console.ErrNotAConsole:
			continue
		default:
			// should not happen
			return errors.Wrap(err, "unable to get console")
		}
	}
	// If all streams are redirected, but we still have a controlling
	// terminal, it can be obtained by opening /dev/tty.
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return err
	}
	c, err := console.ConsoleFromFile(tty)
	if err != nil {
		return errors.Wrap(err, "unable to get console")
	}

	t.hostConsole = c
	return nil
}

func (t *tty) recvtty(process *libcontainer.Process, socket *os.File) (Err error) {
	f, err := utils.RecvFd(socket)
	if err != nil {
		return err
	}
	cons, err := console.ConsoleFromFile(f)
	if err != nil {
		return err
	}
	err = console.ClearONLCR(cons.Fd())
	if err != nil {
		return err
	}
	epoller, err := console.NewEpoller()
	if err != nil {
		return err
	}
	epollConsole, err := epoller.Add(cons)
	if err != nil {
		return err
	}
	defer func() {
		if Err != nil {
			epollConsole.Close()
		}
	}()
	go epoller.Wait()
	go io.Copy(epollConsole, os.Stdin)
	t.wg.Add(1)
	go t.copyIO(os.Stdout, epollConsole)

	// Set raw mode for the controlling terminal.
	if err := t.hostConsole.SetRaw(); err != nil {
		return fmt.Errorf("failed to set the terminal from the stdin: %v", err)
	}
	go handleInterrupt(t.hostConsole)

	t.epoller = epoller
	t.console = epollConsole
	t.closers = []io.Closer{epollConsole}
	return nil
}

func handleInterrupt(c console.Console) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)
	<-sigchan
	c.Reset()
	os.Exit(0)
}

func (t *tty) waitConsole() error {
	if t.consoleC != nil {
		return <-t.consoleC
	}
	return nil
}

// ClosePostStart closes any fds that are provided to the container and dup2'd
// so that we no longer have copy in our process.
func (t *tty) ClosePostStart() error {
	for _, c := range t.postStart {
		c.Close()
	}
	return nil
}

// Close closes all open fds for the tty and/or restores the original
// stdin state to what it was prior to the container execution
func (t *tty) Close() error {
	// ensure that our side of the fds are always closed
	for _, c := range t.postStart {
		c.Close()
	}
	// the process is gone at this point, shutting down the console if we have
	// one and wait for all IO to be finished
	if t.console != nil && t.epoller != nil {
		t.console.Shutdown(t.epoller.CloseConsole)
	}
	t.wg.Wait()
	for _, c := range t.closers {
		c.Close()
	}
	if t.hostConsole != nil {
		t.hostConsole.Reset()
	}
	return nil
}

func (t *tty) resize() error {
	if t.console == nil || t.hostConsole == nil {
		return nil
	}
	return t.console.ResizeFrom(t.hostConsole)
}
