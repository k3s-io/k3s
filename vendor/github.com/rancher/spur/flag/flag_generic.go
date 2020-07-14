// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flag

import (
	"fmt"
	"time"

	"github.com/rancher/spur/generic"
)

var _ = time.Time{}

// GenericValue takes a pointer to a generic type
type GenericValue struct {
	ptr interface{}
	set bool
}

// NewGenericValue returns a flag.Value given a pointer
func NewGenericValue(ptr interface{}) Value {
	generic.PtrPanic(ptr)
	return &GenericValue{ptr: ptr}
}

// Get returns the contents of the stored pointer
func (v *GenericValue) Get() interface{} {
	return generic.ValueOfPtr(v.ptr)
}

// Set will convert a given value to the type of our pointer
// and store the new value
func (v *GenericValue) Set(value interface{}) error {
	if generic.IsSlice(v.Get()) && !v.set {
		// If this is a slice and has not already been set then
		// clear any existing value
		generic.Set(v.ptr, generic.Zero(v.Get()))
		v.set = true
	}
	val, err := generic.Convert(v.Get(), value)
	if err != nil {
		return err
	}
	generic.Set(v.ptr, val)
	return nil
}

// String returns a string representation of our generic value
func (v *GenericValue) String() string {
	return generic.Stringify(v.Get())
}

// GenericVar defines a generic flag with specified name, default value, and usage string.
// The argument p points to a generic variable in which to store the value of the flag.
func (f *FlagSet) GenericVar(ptr interface{}, name string, value interface{}, usage string) {
	generic.Set(ptr, value)
	f.Var(NewGenericValue(ptr), name, usage)
}

// Generic defines a generic flag with specified name, default value, and usage string.
// The return value is the address of a generic variable that stores the value of the flag.
func (f *FlagSet) Generic(name string, value interface{}, usage string) interface{} {
	if value == nil {
		panic(fmt.Errorf("creating generic from nil interface %s", name))
	}
	ptr := generic.New(value)
	f.GenericVar(ptr, name, value, usage)
	return ptr
}

// GenericVar defines a generic flag with specified name, default value, and usage string.
// The argument p points to a generic variable in which to store the value of the flag.
func GenericVar(ptr interface{}, name string, value interface{}, usage string) {
	CommandLine.GenericVar(ptr, name, value, usage)
}

// Generic defines a generic flag with specified name, default value, and usage string.
// The return value is the address of a generic variable that stores the value of the flag.
func Generic(name string, value interface{}, usage string) interface{} {
	return CommandLine.Generic(name, value, usage)
}
