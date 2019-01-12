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
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/containerd/containerd/cio"
	"github.com/containerd/fifo"
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// AttachOptions specifies how to attach to a container.
type AttachOptions struct {
	Stdin     io.Reader
	Stdout    io.WriteCloser
	Stderr    io.WriteCloser
	Tty       bool
	StdinOnce bool
	// CloseStdin is the function to close container stdin.
	CloseStdin func() error
}

// StreamType is the type of the stream, stdout/stderr.
type StreamType string

const (
	// Stdin stream type.
	Stdin StreamType = "stdin"
	// Stdout stream type.
	Stdout StreamType = StreamType(runtime.Stdout)
	// Stderr stream type.
	Stderr StreamType = StreamType(runtime.Stderr)
)

type wgCloser struct {
	ctx    context.Context
	wg     *sync.WaitGroup
	set    []io.Closer
	cancel context.CancelFunc
}

func (g *wgCloser) Wait() {
	g.wg.Wait()
}

func (g *wgCloser) Close() {
	for _, f := range g.set {
		f.Close()
	}
}

func (g *wgCloser) Cancel() {
	g.cancel()
}

// newFifos creates fifos directory for a container.
func newFifos(root, id string, tty, stdin bool) (*cio.FIFOSet, error) {
	root = filepath.Join(root, "io")
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	fifos, err := cio.NewFIFOSetInDir(root, id, tty)
	if err != nil {
		return nil, err
	}
	if !stdin {
		fifos.Stdin = ""
	}
	return fifos, nil
}

type stdioPipes struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// newStdioPipes creates actual fifos for stdio.
func newStdioPipes(fifos *cio.FIFOSet) (_ *stdioPipes, _ *wgCloser, err error) {
	var (
		f           io.ReadWriteCloser
		set         []io.Closer
		ctx, cancel = context.WithCancel(context.Background())
		p           = &stdioPipes{}
	)
	defer func() {
		if err != nil {
			for _, f := range set {
				f.Close()
			}
			cancel()
		}
	}()

	if fifos.Stdin != "" {
		if f, err = fifo.OpenFifo(ctx, fifos.Stdin, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); err != nil {
			return nil, nil, err
		}
		p.stdin = f
		set = append(set, f)
	}

	if f, err = fifo.OpenFifo(ctx, fifos.Stdout, syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); err != nil {
		return nil, nil, err
	}
	p.stdout = f
	set = append(set, f)

	if f, err = fifo.OpenFifo(ctx, fifos.Stderr, syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); err != nil {
		return nil, nil, err
	}
	p.stderr = f
	set = append(set, f)

	return p, &wgCloser{
		wg:     &sync.WaitGroup{},
		set:    set,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}
