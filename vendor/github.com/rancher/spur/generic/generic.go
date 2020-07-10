// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"gopkg.in/yaml.v2"
)

// Marshal is the function used for marshaling slices
var Marshal = json.Marshal

// Unmarshal is the function used for un-marshaling slices
var Unmarshal = yaml.Unmarshal

// ToStringFunc is the function definition for converting types to strings
type ToStringFunc = func(interface{}) (string, bool)

// FromStringFunc is the function definition for converting strings to types
type FromStringFunc = func(string) (interface{}, error)

// ToStringMap provides a mapping of type to string conversion function
var ToStringMap = map[string]ToStringFunc{}

// FromStringMap provides a mapping of string to type conversion function
var FromStringMap = map[string]FromStringFunc{}

// TimeLayouts provides a list of layouts to attempt when converting time strings
var TimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	time.UnixDate,
	time.RubyDate,
	time.ANSIC,
	time.RFC822,
	time.RFC822Z,
	time.RFC850,
	time.RFC1123,
	time.RFC1123Z,
	time.StampNano,
	time.StampMicro,
	time.StampMilli,
	time.Stamp,
	time.Kitchen,
}

// ToString is a convenience function for converting types to strings as defined in ToStringMap
func ToString(value interface{}) (string, bool) {
	if value == nil {
		return "", false
	}
	if toString := ToStringMap[TypeOf(value).String()]; toString != nil {
		return toString(value)
	}
	return "", false
}

// FromString is a convenience function for converting strings to types as defined in FromStringMap
func FromString(value string, ptr interface{}) error {
	PtrPanic(ptr)
	typ := reflect.TypeOf(ptr).Elem().String()
	fromString := FromStringMap[typ]
	if fromString == nil {
		return errParse
	}
	val, err := fromString(value)
	if err != nil {
		return numError(err)
	}
	Set(ptr, val)
	return nil
}

// TypeOf returns the dereferenced value's type
func TypeOf(value interface{}) reflect.Type {
	typ := reflect.TypeOf(value)
	if typ != nil && typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return typ
}

// ElemTypeOf returns the dereferenced value's type or TypeOf is not an Elem
func ElemTypeOf(value interface{}) reflect.Type {
	typ := TypeOf(value)
	if typ.Kind() == reflect.Slice {
		return typ.Elem()
	}
	return typ
}

// New returns a new reflection with TypeOf value
func New(value interface{}) interface{} {
	return reflect.New(TypeOf(value)).Interface()
}

// NewElem returns a new reflection with ElemTypeOf value
func NewElem(value interface{}) interface{} {
	return reflect.New(ElemTypeOf(value)).Interface()
}

// Zero returns a zero reflection with TypeOf value
func Zero(value interface{}) interface{} {
	return reflect.Zero(TypeOf(value)).Interface()
}

// IsSlice return true if the TypeOf value is a slice
func IsSlice(value interface{}) bool {
	if value == nil {
		return false
	}
	return TypeOf(value).Kind() == reflect.Slice
}

// PtrPanic halts execution if the passed ptr is not a pointer
func PtrPanic(ptr interface{}) {
	if !IsPtr(ptr) {
		panic(fmt.Errorf("expected pointer type, got %s", reflect.TypeOf(ptr).String()))
	}
}

// Set will assign the contents of ptr to value
func Set(ptr interface{}, value interface{}) {
	PtrPanic(ptr)
	if value == nil {
		return
	}
	reflect.ValueOf(ptr).Elem().Set(reflect.ValueOf(value))
}

// Len returns the length of a slice, or -1 if not a slice
func Len(value interface{}) int {
	if !IsSlice(value) {
		return -1
	}
	return reflect.ValueOf(value).Len()
}

// Index will return the value of a slice at a given index
func Index(value interface{}, i int) interface{} {
	if !IsSlice(value) {
		return nil
	}
	return reflect.ValueOf(value).Index(i).Interface()
}

// Append will append an element onto a generic slice
func Append(slice interface{}, elem interface{}) interface{} {
	return reflect.Append(reflect.ValueOf(slice), reflect.ValueOf(elem)).Interface()
}

// IsPtr returns true if the given value is of kind reflect.Ptr
func IsPtr(value interface{}) bool {
	if value == nil {
		return false
	}
	return reflect.TypeOf(value).Kind() == reflect.Ptr
}

// ValueOfPtr returns the contents of a pointer, or the given value if not a pointer
func ValueOfPtr(value interface{}) interface{} {
	if !IsPtr(value) {
		return value
	}
	elem := reflect.ValueOf(value).Elem()
	if !elem.IsValid() {
		return nil
	}
	return elem.Interface()
}

// Convert will return a new result of type src, where value is converted to the type
// of src or appended if src is a slice and value is an element
func Convert(src interface{}, value interface{}) (interface{}, error) {
	// Convert an element
	elem, err := ConvertElem(src, value)
	if !IsSlice(src) {
		// Return value and error if not a slice
		return elem, err
	}
	// Try deserializing as string
	if s, ok := value.(string); ok {
		val := New(src)
		if err := Unmarshal([]byte(s), val); err == nil {
			return ValueOfPtr(val), nil
		}
	}
	// If no error from converting element return appended value
	if err == nil {
		return Append(src, elem), nil
	}
	// Try evaluating value as a slice of interfaces
	otherValue, ok := value.([]interface{})
	if !ok {
		return nil, errParse
	}
	// Create a new slice and append each converted element
	slice := Zero(src)
	for _, other := range otherValue {
		elem, err := ConvertElem(src, other)
		if err != nil {
			return nil, err
		}
		slice = Append(slice, elem)
	}
	return slice, nil
}

// ConvertElem will return a new result, where value is converted to the type
// of src or returned as an element if src is a slice
func ConvertElem(src interface{}, value interface{}) (interface{}, error) {
	// Get our value as a string
	s, ok := value.(string)
	if !ok {
		if s, ok = ToString(value); !ok {
			return nil, errParse
		}
	}
	// Return a new value from the string
	ptr := NewElem(src)
	err := FromString(s, ptr)
	return ValueOfPtr(ptr), err
}

// Stringify returns the ToString version of the value, or the Marshaled version
// in the case of slices, otherwise panic if cannot be converted to string
func Stringify(value interface{}) string {
	if s, ok := ToString(value); ok {
		return s
	}
	if b, err := Marshal(value); err == nil {
		return string(b)
	}
	panic(errParse)
}
