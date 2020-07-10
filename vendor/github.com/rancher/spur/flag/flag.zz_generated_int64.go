// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"time"
)

var _ = time.Time{}

// Int64Var defines a int64 flag with specified name, default value, and usage string.
// The argument p points to a int64 variable in which to store the value of the flag.
func (f *FlagSet) Int64Var(ptr *int64, name string, value int64, usage string) {
	f.GenericVar(ptr, name, value, usage)
}

// Int64 defines a int64 flag with specified name, default value, and usage string.
// The return value is the address of a int64 variable that stores the value of the flag.
func (f *FlagSet) Int64(name string, value int64, usage string) *int64 {
	return f.Generic(name, value, usage).(*int64)
}

// Int64Var defines a int64 flag with specified name, default value, and usage string.
// The argument p points to a int64 variable in which to store the value of the flag.
func Int64Var(ptr *int64, name string, value int64, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// Int64 defines a int64 flag with specified name, default value, and usage string.
// The return value is the address of a int64 variable that stores the value of the flag.
func Int64(name string, value int64, usage string) *int64 {
	return CommandLine.Generic(name, value, usage).(*int64)
}
