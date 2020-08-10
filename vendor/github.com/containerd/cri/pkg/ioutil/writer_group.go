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

package ioutil

import (
	"errors"
	"io"
	"sync"
)

// WriterGroup is a group of writers. Writer could be dynamically
// added and removed.
type WriterGroup struct {
	mu      sync.Mutex
	writers map[string]io.WriteCloser
	closed  bool
}

var _ io.Writer = &WriterGroup{}

// NewWriterGroup creates an empty writer group.
func NewWriterGroup() *WriterGroup {
	return &WriterGroup{
		writers: make(map[string]io.WriteCloser),
	}
}

// Add adds a writer into the group. The writer will be closed
// if the writer group is closed.
func (g *WriterGroup) Add(key string, w io.WriteCloser) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		w.Close()
		return
	}
	g.writers[key] = w
}

// Get gets a writer from the group, returns nil if the writer
// doesn't exist.
func (g *WriterGroup) Get(key string) io.WriteCloser {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.writers[key]
}

// Remove removes a writer from the group.
func (g *WriterGroup) Remove(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	w, ok := g.writers[key]
	if !ok {
		return
	}
	w.Close()
	delete(g.writers, key)
}

// Write writes data into each writer. If a writer returns error,
// it will be closed and removed from the writer group. It returns
// error if writer group is empty.
func (g *WriterGroup) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for k, w := range g.writers {
		n, err := w.Write(p)
		if err == nil && len(p) == n {
			continue
		}
		// The writer is closed or in bad state, remove it.
		w.Close()
		delete(g.writers, k)
	}
	if len(g.writers) == 0 {
		return 0, errors.New("writer group is empty")
	}
	return len(p), nil
}

// Close closes the writer group. Write will return error after
// closed.
func (g *WriterGroup) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, w := range g.writers {
		w.Close()
	}
	g.writers = nil
	g.closed = true
}
