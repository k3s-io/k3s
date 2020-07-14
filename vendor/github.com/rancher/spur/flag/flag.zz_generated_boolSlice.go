// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"time"
)

var _ = time.Time{}

// BoolSliceVar defines a []bool flag with specified name, default value, and usage string.
// The argument p points to a []bool variable in which to store the value of the flag.
func (f *FlagSet) BoolSliceVar(ptr *[]bool, name string, value []bool, usage string) {
	f.GenericVar(ptr, name, value, usage)
}

// BoolSlice defines a []bool flag with specified name, default value, and usage string.
// The return value is the address of a []bool variable that stores the value of the flag.
func (f *FlagSet) BoolSlice(name string, value []bool, usage string) *[]bool {
	return f.Generic(name, value, usage).(*[]bool)
}

// BoolSliceVar defines a []bool flag with specified name, default value, and usage string.
// The argument p points to a []bool variable in which to store the value of the flag.
func BoolSliceVar(ptr *[]bool, name string, value []bool, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// BoolSlice defines a []bool flag with specified name, default value, and usage string.
// The return value is the address of a []bool variable that stores the value of the flag.
func BoolSlice(name string, value []bool, usage string) *[]bool {
	return CommandLine.Generic(name, value, usage).(*[]bool)
}
