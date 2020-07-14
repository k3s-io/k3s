// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"time"
)

var _ = time.Time{}

// IntVar defines a int flag with specified name, default value, and usage string.
// The argument p points to a int variable in which to store the value of the flag.
func (f *FlagSet) IntVar(ptr *int, name string, value int, usage string) {
	f.GenericVar(ptr, name, value, usage)
}

// Int defines a int flag with specified name, default value, and usage string.
// The return value is the address of a int variable that stores the value of the flag.
func (f *FlagSet) Int(name string, value int, usage string) *int {
	return f.Generic(name, value, usage).(*int)
}

// IntVar defines a int flag with specified name, default value, and usage string.
// The argument p points to a int variable in which to store the value of the flag.
func IntVar(ptr *int, name string, value int, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// Int defines a int flag with specified name, default value, and usage string.
// The return value is the address of a int variable that stores the value of the flag.
func Int(name string, value int, usage string) *int {
	return CommandLine.Generic(name, value, usage).(*int)
}
