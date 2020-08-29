// +build windows

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

package io

import (
	"io"
	"net"
	"os"
	"sync"

	winio "github.com/Microsoft/go-winio"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type pipe struct {
	l      net.Listener
	con    net.Conn
	conErr error
	conWg  sync.WaitGroup
}

func openPipe(ctx context.Context, fn string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	l, err := winio.ListenPipe(fn, nil)
	if err != nil {
		return nil, err
	}
	p := &pipe{l: l}
	p.conWg.Add(1)
	go func() {
		defer p.conWg.Done()
		c, err := l.Accept()
		if err != nil {
			p.conErr = err
			return
		}
		p.con = c
	}()
	return p, nil
}

func (p *pipe) Write(b []byte) (int, error) {
	p.conWg.Wait()
	if p.conErr != nil {
		return 0, errors.Wrap(p.conErr, "connection error")
	}
	return p.con.Write(b)
}

func (p *pipe) Read(b []byte) (int, error) {
	p.conWg.Wait()
	if p.conErr != nil {
		return 0, errors.Wrap(p.conErr, "connection error")
	}
	return p.con.Read(b)
}

func (p *pipe) Close() error {
	p.l.Close()
	p.conWg.Wait()
	if p.con != nil {
		return p.con.Close()
	}
	return p.conErr
}
