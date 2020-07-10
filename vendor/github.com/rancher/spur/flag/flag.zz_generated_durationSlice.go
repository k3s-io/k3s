// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"time"
)

var _ = time.Time{}

// DurationSliceVar defines a []time.Duration flag with specified name, default value, and usage string.
// The argument p points to a []time.Duration variable in which to store the value of the flag.
func (f *FlagSet) DurationSliceVar(ptr *[]time.Duration, name string, value []time.Duration, usage string) {
	f.GenericVar(ptr, name, value, usage)
}

// DurationSlice defines a []time.Duration flag with specified name, default value, and usage string.
// The return value is the address of a []time.Duration variable that stores the value of the flag.
func (f *FlagSet) DurationSlice(name string, value []time.Duration, usage string) *[]time.Duration {
	return f.Generic(name, value, usage).(*[]time.Duration)
}

// DurationSliceVar defines a []time.Duration flag with specified name, default value, and usage string.
// The argument p points to a []time.Duration variable in which to store the value of the flag.
func DurationSliceVar(ptr *[]time.Duration, name string, value []time.Duration, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// DurationSlice defines a []time.Duration flag with specified name, default value, and usage string.
// The return value is the address of a []time.Duration variable that stores the value of the flag.
func DurationSlice(name string, value []time.Duration, usage string) *[]time.Duration {
	return CommandLine.Generic(name, value, usage).(*[]time.Duration)
}
