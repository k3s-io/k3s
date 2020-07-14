// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"time"
)

var _ = time.Time{}

// TimeSliceVar defines a []time.Time flag with specified name, default value, and usage string.
// The argument p points to a []time.Time variable in which to store the value of the flag.
func (f *FlagSet) TimeSliceVar(ptr *[]time.Time, name string, value []time.Time, usage string) {
	f.GenericVar(ptr, name, value, usage)
}

// TimeSlice defines a []time.Time flag with specified name, default value, and usage string.
// The return value is the address of a []time.Time variable that stores the value of the flag.
func (f *FlagSet) TimeSlice(name string, value []time.Time, usage string) *[]time.Time {
	return f.Generic(name, value, usage).(*[]time.Time)
}

// TimeSliceVar defines a []time.Time flag with specified name, default value, and usage string.
// The argument p points to a []time.Time variable in which to store the value of the flag.
func TimeSliceVar(ptr *[]time.Time, name string, value []time.Time, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// TimeSlice defines a []time.Time flag with specified name, default value, and usage string.
// The return value is the address of a []time.Time variable that stores the value of the flag.
func TimeSlice(name string, value []time.Time, usage string) *[]time.Time {
	return CommandLine.Generic(name, value, usage).(*[]time.Time)
}
