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
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"time"

	"github.com/sirupsen/logrus"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	cioutil "github.com/containerd/cri/pkg/ioutil"
)

const (
	// delimiter used in CRI logging format.
	delimiter = ' '
	// eof is end-of-line.
	eol = '\n'
	// timestampFormat is the timestamp format used in CRI logging format.
	timestampFormat = time.RFC3339Nano
	// defaultBufSize is the default size of the read buffer in bytes.
	defaultBufSize = 4096
)

// NewDiscardLogger creates logger which discards all the input.
func NewDiscardLogger() io.WriteCloser {
	return cioutil.NewNopWriteCloser(ioutil.Discard)
}

// NewCRILogger returns a write closer which redirect container log into
// log file, and decorate the log line into CRI defined format. It also
// returns a channel which indicates whether the logger is stopped.
// maxLen is the max length limit of a line. A line longer than the
// limit will be cut into multiple lines.
func NewCRILogger(path string, w io.Writer, stream StreamType, maxLen int) (io.WriteCloser, <-chan struct{}) {
	logrus.Debugf("Start writing stream %q to log file %q", stream, path)
	prc, pwc := io.Pipe()
	stop := make(chan struct{})
	go func() {
		redirectLogs(path, prc, w, stream, maxLen)
		close(stop)
	}()
	return pwc, stop
}

func redirectLogs(path string, rc io.ReadCloser, w io.Writer, s StreamType, maxLen int) {
	defer rc.Close()
	var (
		stream    = []byte(s)
		delimiter = []byte{delimiter}
		partial   = []byte(runtime.LogTagPartial)
		full      = []byte(runtime.LogTagFull)
		buf       [][]byte
		length    int
		bufSize   = defaultBufSize
	)
	// Make sure bufSize <= maxLen
	if maxLen > 0 && maxLen < bufSize {
		bufSize = maxLen
	}
	r := bufio.NewReaderSize(rc, bufSize)
	writeLine := func(tag, line []byte) {
		timestamp := time.Now().AppendFormat(nil, timestampFormat)
		data := bytes.Join([][]byte{timestamp, stream, tag, line}, delimiter)
		data = append(data, eol)
		if _, err := w.Write(data); err != nil {
			logrus.WithError(err).Errorf("Fail to write %q log to log file %q", s, path)
			// Continue on write error to drain the container output.
		}
	}
	for {
		var stop bool
		newLine, isPrefix, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				logrus.Debugf("Getting EOF from stream %q while redirecting to log file %q", s, path)
			} else {
				logrus.WithError(err).Errorf("An error occurred when redirecting stream %q to log file %q", s, path)
			}
			if length == 0 {
				// No content left to write, break.
				break
			}
			// Stop after writing the content left in buffer.
			stop = true
		} else {
			// Buffer returned by ReadLine will change after
			// next read, copy it.
			l := make([]byte, len(newLine))
			copy(l, newLine)
			buf = append(buf, l)
			length += len(l)
		}
		if maxLen > 0 && length > maxLen {
			exceedLen := length - maxLen
			last := buf[len(buf)-1]
			if exceedLen > len(last) {
				// exceedLen must <= len(last), or else the buffer
				// should have be written in the previous iteration.
				panic("exceed length should <= last buffer size")
			}
			buf[len(buf)-1] = last[:len(last)-exceedLen]
			writeLine(partial, bytes.Join(buf, nil))
			buf = [][]byte{last[len(last)-exceedLen:]}
			length = exceedLen
		}
		if isPrefix {
			continue
		}
		writeLine(full, bytes.Join(buf, nil))
		buf = nil
		length = 0
		if stop {
			break
		}
	}
	logrus.Debugf("Finish redirecting stream %q to log file %q", s, path)
}
