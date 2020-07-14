// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"time"
)

var _ = time.Time{}

// Uint64SliceVar defines a []uint64 flag with specified name, default value, and usage string.
// The argument p points to a []uint64 variable in which to store the value of the flag.
func (f *FlagSet) Uint64SliceVar(ptr *[]uint64, name string, value []uint64, usage string) {
	f.GenericVar(ptr, name, value, usage)
}

// Uint64Slice defines a []uint64 flag with specified name, default value, and usage string.
// The return value is the address of a []uint64 variable that stores the value of the flag.
func (f *FlagSet) Uint64Slice(name string, value []uint64, usage string) *[]uint64 {
	return f.Generic(name, value, usage).(*[]uint64)
}

// Uint64SliceVar defines a []uint64 flag with specified name, default value, and usage string.
// The argument p points to a []uint64 variable in which to store the value of the flag.
func Uint64SliceVar(ptr *[]uint64, name string, value []uint64, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// Uint64Slice defines a []uint64 flag with specified name, default value, and usage string.
// The return value is the address of a []uint64 variable that stores the value of the flag.
func Uint64Slice(name string, value []uint64, usage string) *[]uint64 {
	return CommandLine.Generic(name, value, usage).(*[]uint64)
}
