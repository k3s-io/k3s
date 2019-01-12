/*
Copyright 2017 The Kubernetes Authors.

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

package io

import (
	"io"
	"sync"

	"github.com/containerd/containerd/cio"
	"github.com/sirupsen/logrus"

	cioutil "github.com/containerd/cri/pkg/ioutil"
)

// ExecIO holds the exec io.
type ExecIO struct {
	id    string
	fifos *cio.FIFOSet
	*stdioPipes
	closer *wgCloser
}

var _ cio.IO = &ExecIO{}

// NewExecIO creates exec io.
func NewExecIO(id, root string, tty, stdin bool) (*ExecIO, error) {
	fifos, err := newFifos(root, id, tty, stdin)
	if err != nil {
		return nil, err
	}
	stdio, closer, err := newStdioPipes(fifos)
	if err != nil {
		return nil, err
	}
	return &ExecIO{
		id:         id,
		fifos:      fifos,
		stdioPipes: stdio,
		closer:     closer,
	}, nil
}

// Config returns io config.
func (e *ExecIO) Config() cio.Config {
	return e.fifos.Config
}

// Attach attaches exec stdio. The logic is similar with container io attach.
func (e *ExecIO) Attach(opts AttachOptions) <-chan struct{} {
	var wg sync.WaitGroup
	var stdinStreamRC io.ReadCloser
	if e.stdin != nil && opts.Stdin != nil {
		stdinStreamRC = cioutil.NewWrapReadCloser(opts.Stdin)
		wg.Add(1)
		go func() {
			if _, err := io.Copy(e.stdin, stdinStreamRC); err != nil {
				logrus.WithError(err).Errorf("Failed to redirect stdin for container exec %q", e.id)
			}
			logrus.Infof("Container exec %q stdin closed", e.id)
			if opts.StdinOnce && !opts.Tty {
				e.stdin.Close()
				if err := opts.CloseStdin(); err != nil {
					logrus.WithError(err).Errorf("Failed to close stdin for container exec %q", e.id)
				}
			} else {
				if e.stdout != nil {
					e.stdout.Close()
				}
				if e.stderr != nil {
					e.stderr.Close()
				}
			}
			wg.Done()
		}()
	}

	attachOutput := func(t StreamType, stream io.WriteCloser, out io.ReadCloser) {
		if _, err := io.Copy(stream, out); err != nil {
			logrus.WithError(err).Errorf("Failed to pipe %q for container exec %q", t, e.id)
		}
		out.Close()
		stream.Close()
		if stdinStreamRC != nil {
			stdinStreamRC.Close()
		}
		e.closer.wg.Done()
		wg.Done()
		logrus.Infof("Finish piping %q of container exec %q", t, e.id)
	}

	if opts.Stdout != nil {
		wg.Add(1)
		// Closer should wait for this routine to be over.
		e.closer.wg.Add(1)
		go attachOutput(Stdout, opts.Stdout, e.stdout)
	}

	if !opts.Tty && opts.Stderr != nil {
		wg.Add(1)
		// Closer should wait for this routine to be over.
		e.closer.wg.Add(1)
		go attachOutput(Stderr, opts.Stderr, e.stderr)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	return done
}

// Cancel cancels exec io.
func (e *ExecIO) Cancel() {
	e.closer.Cancel()
}

// Wait waits exec io to finish.
func (e *ExecIO) Wait() {
	e.closer.Wait()
}

// Close closes all FIFOs.
func (e *ExecIO) Close() error {
	if e.closer != nil {
		e.closer.Close()
	}
	if e.fifos != nil {
		return e.fifos.Close()
	}
	return nil
}
