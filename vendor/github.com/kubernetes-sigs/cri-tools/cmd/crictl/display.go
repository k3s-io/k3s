/*
Copyright 2019 The Kubernetes Authors.

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

package crictl

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

const (
	columnContainer  = "CONTAINER"
	columnImage      = "IMAGE"
	columnImageID    = "IMAGE ID"
	columnCreated    = "CREATED"
	columnState      = "STATE"
	columnName       = "NAME"
	columnAttempt    = "ATTEMPT"
	columnPodID      = "POD ID"
	columnPodRuntime = "RUNTIME"
	columnNamespace  = "NAMESPACE"
	columnSize       = "SIZE"
	columnTag        = "TAG"
	columnDigest     = "DIGEST"
	columnMemory     = "MEM"
	columnInodes     = "INODES"
	columnDisk       = "DISK"
	columnCPU        = "CPU %"
)

// display use to output something on screen with table format.
type display struct {
	w *tabwriter.Writer
}

// newTableDisplay creates a display instance, and uses to format output with table.
func newTableDisplay(minwidth, tabwidth, padding int, padchar byte, flags uint) *display {
	w := tabwriter.NewWriter(os.Stdout, minwidth, tabwidth, padding, padchar, 0)
	return &display{w}
}

// AddRow add a row of data.
func (d *display) AddRow(row []string) {
	fmt.Fprintln(d.w, strings.Join(row, "\t"))
}

// Flush output all rows on screen.
func (d *display) Flush() error {
	return d.w.Flush()
}

// ClearScreen clear all output on screen.
func (d *display) ClearScreen() {
	fmt.Fprint(os.Stdout, "\033[2J")
	fmt.Fprint(os.Stdout, "\033[H")
}
