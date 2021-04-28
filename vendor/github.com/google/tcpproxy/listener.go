// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcpproxy

import (
	"io"
	"net"
	"sync"
)

// TargetListener implements both net.Listener and Target.
// Matched Targets become accepted connections.
type TargetListener struct {
	Address string // Address is the string reported by TargetListener.Addr().String().

	mu       sync.Mutex
	cond     *sync.Cond
	closed   bool
	nextConn net.Conn
}

var (
	_ net.Listener = (*TargetListener)(nil)
	_ Target       = (*TargetListener)(nil)
)

func (tl *TargetListener) lock() {
	tl.mu.Lock()
	if tl.cond == nil {
		tl.cond = sync.NewCond(&tl.mu)
	}
}

type tcpAddr string

func (a tcpAddr) Network() string { return "tcp" }
func (a tcpAddr) String() string  { return string(a) }

// Addr returns the listener's Address field as a net.Addr.
func (tl *TargetListener) Addr() net.Addr { return tcpAddr(tl.Address) }

// Close stops listening for new connections. All new connections
// routed to this listener will be closed. Already accepted
// connections are not closed.
func (tl *TargetListener) Close() error {
	tl.lock()
	if tl.closed {
		tl.mu.Unlock()
		return nil
	}
	tl.closed = true
	tl.mu.Unlock()
	tl.cond.Broadcast()
	return nil
}

// HandleConn implements the Target interface. It blocks until tl is
// closed or another goroutine has called Accept and received c.
func (tl *TargetListener) HandleConn(c net.Conn) {
	tl.lock()
	defer tl.mu.Unlock()
	for tl.nextConn != nil && !tl.closed {
		tl.cond.Wait()
	}
	if tl.closed {
		c.Close()
		return
	}
	tl.nextConn = c
	tl.cond.Broadcast() // Signal might be sufficient; verify.
	for tl.nextConn == c && !tl.closed {
		tl.cond.Wait()
	}
	if tl.closed {
		c.Close()
		return
	}
}

// Accept implements the Accept method in the net.Listener interface.
func (tl *TargetListener) Accept() (net.Conn, error) {
	tl.lock()
	for tl.nextConn == nil && !tl.closed {
		tl.cond.Wait()
	}
	if tl.closed {
		tl.mu.Unlock()
		return nil, io.EOF
	}
	c := tl.nextConn
	tl.nextConn = nil
	tl.mu.Unlock()
	tl.cond.Broadcast() // Signal might be sufficient; verify.

	return c, nil
}
