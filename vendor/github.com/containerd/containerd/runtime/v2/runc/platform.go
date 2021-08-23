// +build linux

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package runc

import (
	"context"
	"io"
	"net/url"
	"os"
	"sync"
	"syscall"

	"github.com/containerd/console"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/process"
	"github.com/containerd/containerd/pkg/stdio"
	"github.com/containerd/fifo"
	"github.com/pkg/errors"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		// setting to 4096 to align with PIPE_BUF
		// http://man7.org/linux/man-pages/man7/pipe.7.html
		buffer := make([]byte, 4096)
		return &buffer
	},
}

// NewPlatform returns a linux platform for use with I/O operations
func NewPlatform() (stdio.Platform, error) {
	epoller, err := console.NewEpoller()
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize epoller")
	}
	go epoller.Wait()
	return &linuxPlatform{
		epoller: epoller,
	}, nil
}

type linuxPlatform struct {
	epoller *console.Epoller
}

func (p *linuxPlatform) CopyConsole(ctx context.Context, console console.Console, id, stdin, stdout, stderr string, wg *sync.WaitGroup) (cons console.Console, retErr error) {
	if p.epoller == nil {
		return nil, errors.New("uninitialized epoller")
	}

	epollConsole, err := p.epoller.Add(console)
	if err != nil {
		return nil, err
	}

	var cwg sync.WaitGroup
	if stdin != "" {
		in, err := fifo.OpenFifo(context.Background(), stdin, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		cwg.Add(1)
		go func() {
			cwg.Done()
			bp := bufPool.Get().(*[]byte)
			defer bufPool.Put(bp)
			io.CopyBuffer(epollConsole, in, *bp)
			// we need to shutdown epollConsole when pipe broken
			epollConsole.Shutdown(p.epoller.CloseConsole)
			epollConsole.Close()
		}()
	}

	uri, err := url.Parse(stdout)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse stdout uri")
	}

	switch uri.Scheme {
	case "binary":
		ns, err := namespaces.NamespaceRequired(ctx)
		if err != nil {
			return nil, err
		}

		cmd := process.NewBinaryCmd(uri, id, ns)

		// In case of unexpected errors during logging binary start, close open pipes
		var filesToClose []*os.File

		defer func() {
			if retErr != nil {
				process.CloseFiles(filesToClose...)
			}
		}()

		// Create pipe to be used by logging binary for Stdout
		outR, outW, err := os.Pipe()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create stdout pipes")
		}
		filesToClose = append(filesToClose, outR)

		// Stderr is created for logging binary but unused when terminal is true
		serrR, _, err := os.Pipe()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create stderr pipes")
		}
		filesToClose = append(filesToClose, serrR)

		r, w, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		filesToClose = append(filesToClose, r)

		cmd.ExtraFiles = append(cmd.ExtraFiles, outR, serrR, w)

		wg.Add(1)
		cwg.Add(1)
		go func() {
			cwg.Done()
			io.Copy(outW, epollConsole)
			outW.Close()
			wg.Done()
		}()

		if err := cmd.Start(); err != nil {
			return nil, errors.Wrap(err, "failed to start logging binary process")
		}

		// Close our side of the pipe after start
		if err := w.Close(); err != nil {
			return nil, errors.Wrap(err, "failed to close write pipe after start")
		}

		// Wait for the logging binary to be ready
		b := make([]byte, 1)
		if _, err := r.Read(b); err != nil && err != io.EOF {
			return nil, errors.Wrap(err, "failed to read from logging binary")
		}
		cwg.Wait()

	default:
		outw, err := fifo.OpenFifo(ctx, stdout, syscall.O_WRONLY, 0)
		if err != nil {
			return nil, err
		}
		outr, err := fifo.OpenFifo(ctx, stdout, syscall.O_RDONLY, 0)
		if err != nil {
			return nil, err
		}
		wg.Add(1)
		cwg.Add(1)
		go func() {
			cwg.Done()
			buf := bufPool.Get().(*[]byte)
			defer bufPool.Put(buf)
			io.CopyBuffer(outw, epollConsole, *buf)

			outw.Close()
			outr.Close()
			wg.Done()
		}()
		cwg.Wait()
	}

	return epollConsole, nil
}

func (p *linuxPlatform) ShutdownConsole(ctx context.Context, cons console.Console) error {
	if p.epoller == nil {
		return errors.New("uninitialized epoller")
	}
	epollConsole, ok := cons.(*console.EpollConsole)
	if !ok {
		return errors.Errorf("expected EpollConsole, got %#v", cons)
	}
	return epollConsole.Shutdown(p.epoller.CloseConsole)
}

func (p *linuxPlatform) Close() error {
	return p.epoller.Close()
}
