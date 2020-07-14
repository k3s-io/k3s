// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"time"
)

var _ = time.Time{}

// Float64SliceVar defines a []float64 flag with specified name, default value, and usage string.
// The argument p points to a []float64 variable in which to store the value of the flag.
func (f *FlagSet) Float64SliceVar(ptr *[]float64, name string, value []float64, usage string) {
	f.GenericVar(ptr, name, value, usage)
}

// Float64Slice defines a []float64 flag with specified name, default value, and usage string.
// The return value is the address of a []float64 variable that stores the value of the flag.
func (f *FlagSet) Float64Slice(name string, value []float64, usage string) *[]float64 {
	return f.Generic(name, value, usage).(*[]float64)
}

// Float64SliceVar defines a []float64 flag with specified name, default value, and usage string.
// The argument p points to a []float64 variable in which to store the value of the flag.
func Float64SliceVar(ptr *[]float64, name string, value []float64, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// Float64Slice defines a []float64 flag with specified name, default value, and usage string.
// The return value is the address of a []float64 variable that stores the value of the flag.
func Float64Slice(name string, value []float64, usage string) *[]float64 {
	return CommandLine.Generic(name, value, usage).(*[]float64)
}
