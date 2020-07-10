// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"time"
)

var _ = time.Time{}

// StringSliceVar defines a []string flag with specified name, default value, and usage string.
// The argument p points to a []string variable in which to store the value of the flag.
func (f *FlagSet) StringSliceVar(ptr *[]string, name string, value []string, usage string) {
	f.GenericVar(ptr, name, value, usage)
}

// StringSlice defines a []string flag with specified name, default value, and usage string.
// The return value is the address of a []string variable that stores the value of the flag.
func (f *FlagSet) StringSlice(name string, value []string, usage string) *[]string {
	return f.Generic(name, value, usage).(*[]string)
}

// StringSliceVar defines a []string flag with specified name, default value, and usage string.
// The argument p points to a []string variable in which to store the value of the flag.
func StringSliceVar(ptr *[]string, name string, value []string, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// StringSlice defines a []string flag with specified name, default value, and usage string.
// The return value is the address of a []string variable that stores the value of the flag.
func StringSlice(name string, value []string, usage string) *[]string {
	return CommandLine.Generic(name, value, usage).(*[]string)
}
