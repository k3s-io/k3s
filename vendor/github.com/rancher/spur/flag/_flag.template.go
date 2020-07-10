// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"time"
)

var _ = time.Time{}

// Title__Var defines a Type__ flag with specified name, default value, and usage string.
// The argument p points to a Type__ variable in which to store the value of the flag.
func (f *FlagSet) Title__Var(ptr *Type__, name string, value Type__, usage string) {
	f.GenericVar(ptr, name, value, usage)
}

// Title__ defines a Type__ flag with specified name, default value, and usage string.
// The return value is the address of a Type__ variable that stores the value of the flag.
func (f *FlagSet) Title__(name string, value Type__, usage string) *Type__ {
	return f.Generic(name, value, usage).(*Type__)
}

// Title__Var defines a Type__ flag with specified name, default value, and usage string.
// The argument p points to a Type__ variable in which to store the value of the flag.
func Title__Var(ptr *Type__, name string, value Type__, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// Title__ defines a Type__ flag with specified name, default value, and usage string.
// The return value is the address of a Type__ variable that stores the value of the flag.
func Title__(name string, value Type__, usage string) *Type__ {
	return CommandLine.Generic(name, value, usage).(*Type__)
}
