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

import "io"

// writeCloseInformer wraps a reader with a close function.
type wrapReadCloser struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

// NewWrapReadCloser creates a wrapReadCloser from a reader.
// NOTE(random-liu): To avoid goroutine leakage, the reader passed in
// must be eventually closed by the caller.
func NewWrapReadCloser(r io.Reader) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.Copy(pw, r)
		pr.Close()
		pw.Close()
	}()
	return &wrapReadCloser{
		reader: pr,
		writer: pw,
	}
}

// Read reads up to len(p) bytes into p.
func (w *wrapReadCloser) Read(p []byte) (int, error) {
	n, err := w.reader.Read(p)
	if err == io.ErrClosedPipe {
		return n, io.EOF
	}
	return n, err
}

// Close closes read closer.
func (w *wrapReadCloser) Close() error {
	w.reader.Close()
	w.writer.Close()
	return nil
}
