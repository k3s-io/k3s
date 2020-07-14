// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"time"
)

var _ = time.Time{}

// UintSliceVar defines a []uint flag with specified name, default value, and usage string.
// The argument p points to a []uint variable in which to store the value of the flag.
func (f *FlagSet) UintSliceVar(ptr *[]uint, name string, value []uint, usage string) {
	f.GenericVar(ptr, name, value, usage)
}

// UintSlice defines a []uint flag with specified name, default value, and usage string.
// The return value is the address of a []uint variable that stores the value of the flag.
func (f *FlagSet) UintSlice(name string, value []uint, usage string) *[]uint {
	return f.Generic(name, value, usage).(*[]uint)
}

// UintSliceVar defines a []uint flag with specified name, default value, and usage string.
// The argument p points to a []uint variable in which to store the value of the flag.
func UintSliceVar(ptr *[]uint, name string, value []uint, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// UintSlice defines a []uint flag with specified name, default value, and usage string.
// The return value is the address of a []uint variable that stores the value of the flag.
func UintSlice(name string, value []uint, usage string) *[]uint {
	return CommandLine.Generic(name, value, usage).(*[]uint)
}
