// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"time"
)

var _ = time.Time{}

// TimeVar defines a time.Time flag with specified name, default value, and usage string.
// The argument p points to a time.Time variable in which to store the value of the flag.
func (f *FlagSet) TimeVar(ptr *time.Time, name string, value time.Time, usage string) {
	f.GenericVar(ptr, name, value, usage)
}

// Time defines a time.Time flag with specified name, default value, and usage string.
// The return value is the address of a time.Time variable that stores the value of the flag.
func (f *FlagSet) Time(name string, value time.Time, usage string) *time.Time {
	return f.Generic(name, value, usage).(*time.Time)
}

// TimeVar defines a time.Time flag with specified name, default value, and usage string.
// The argument p points to a time.Time variable in which to store the value of the flag.
func TimeVar(ptr *time.Time, name string, value time.Time, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// Time defines a time.Time flag with specified name, default value, and usage string.
// The return value is the address of a time.Time variable that stores the value of the flag.
func Time(name string, value time.Time, usage string) *time.Time {
	return CommandLine.Generic(name, value, usage).(*time.Time)
}
