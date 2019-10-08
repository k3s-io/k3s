// Copyright 2013, 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package errors

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

var trimValue atomic.Value
var trimDefault = filepath.Join(build.Default.GOPATH, "src") + string(os.PathSeparator)

func trimSourcePath(filename string) string {
	prefix := trimDefault
	if v := trimValue.Load(); v != nil {
		prefix = v.(string)
	}
	return strings.TrimPrefix(filename, prefix)
}

func SetSourceTrimPrefix(s string) string {
	previous := trimDefault
	if v := trimValue.Load(); v != nil {
		previous = v.(string)
	}
	trimValue.Store(s)
	return previous
}
