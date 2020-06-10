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

package progress

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/containerd/console"
)

var (
	regexCleanLine = regexp.MustCompile("\x1b\\[[0-9]+m[\x1b]?")
)

// Writer buffers writes until flush, at which time the last screen is cleared
// and the current buffer contents are written. This is useful for
// implementing progress displays, such as those implemented in docker and
// git.
type Writer struct {
	buf   bytes.Buffer
	w     io.Writer
	lines int
}

// NewWriter returns a writer
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w: w,
	}
}

// Write the provided bytes
func (w *Writer) Write(p []byte) (n int, err error) {
	return w.buf.Write(p)
}

// Flush should be called when refreshing the current display.
func (w *Writer) Flush() error {
	if w.buf.Len() == 0 {
		return nil
	}

	if err := w.clearLines(); err != nil {
		return err
	}
	w.lines = countLines(w.buf.String())

	if _, err := w.w.Write(w.buf.Bytes()); err != nil {
		return err
	}

	w.buf.Reset()
	return nil
}

// TODO(stevvooe): The following are system specific. Break these out if we
// decide to build this package further.

func (w *Writer) clearLines() error {
	for i := 0; i < w.lines; i++ {
		if _, err := fmt.Fprintf(w.w, "\x1b[1A\x1b[2K\r"); err != nil {
			return err
		}
	}

	return nil
}

// countLines in the output. If a line is longer than the console width then
// an extra line is added to the count for each wrapped line. If the console
// width is undefined then 0 is returned so that no lines are cleared on the next
// flush.
func countLines(output string) int {
	con, err := console.ConsoleFromFile(os.Stdin)
	if err != nil {
		return 0
	}
	ws, err := con.Size()
	if err != nil {
		return 0
	}
	width := int(ws.Width)
	if width <= 0 {
		return 0
	}
	strlines := strings.Split(output, "\n")
	lines := -1
	for _, line := range strlines {
		lines += (len(stripLine(line))-1)/width + 1
	}
	return lines
}

func stripLine(line string) string {
	return string(regexCleanLine.ReplaceAll([]byte(line), []byte{}))
}
