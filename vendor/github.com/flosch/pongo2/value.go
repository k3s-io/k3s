package pongo2

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type Value struct {
	val  reflect.Value
	safe bool // used to indicate whether a Value needs explicit escaping in the template
}

// AsValue converts any given value to a pongo2.Value
// Usually being used within own functions passed to a template
// through a Context or within filter functions.
//
// Example:
//     AsValue("my string")
func AsValue(i interface{}) *Value {
	return &Value{
		val: reflect.ValueOf(i),
	}
}

// AsSafeValue works like AsValue, but does not apply the 'escape' filter.
func AsSafeValue(i interface{}) *Value {
	return &Value{
		val:  reflect.ValueOf(i),
		safe: true,
	}
}

func (v *Value) getResolvedValue() reflect.Value {
	if v.val.IsValid() && v.val.Kind() == reflect.Ptr {
		return v.val.Elem()
	}
	return v.val
}

// IsString checks whether the underlying value is a string
func (v *Value) IsString() bool {
	return v.getResolvedValue().Kind() == reflect.String
}

// IsBool checks whether the underlying value is a bool
func (v *Value) IsBool() bool {
	return v.getResolvedValue().Kind() == reflect.Bool
}

// IsFloat checks whether the underlying value is a float
func (v *Value) IsFloat() bool {
	return v.getResolvedValue().Kind() == reflect.Float32 ||
		v.getResolvedValue().Kind() == reflect.Float64
}

// IsInteger checks whether the underlying value is an integer
func (v *Value) IsInteger() bool {
	return v.getResolvedValue().Kind() == reflect.Int ||
		v.getResolvedValue().Kind() == reflect.Int8 ||
		v.getResolvedValue().Kind() == reflect.Int16 ||
		v.getResolvedValue().Kind() == reflect.Int32 ||
		v.getResolvedValue().Kind() == reflect.Int64 ||
		v.getResolvedValue().Kind() == reflect.Uint ||
		v.getResolvedValue().Kind() == reflect.Uint8 ||
		v.getResolvedValue().Kind() == reflect.Uint16 ||
		v.getResolvedValue().Kind() == reflect.Uint32 ||
		v.getResolvedValue().Kind() == reflect.Uint64
}

// IsNumber checks whether the underlying value is either an integer
// or a float.
func (v *Value) IsNumber() bool {
	return v.IsInteger() || v.IsFloat()
}

// IsNil checks whether the underlying value is NIL
func (v *Value) IsNil() bool {
	//fmt.Printf("%+v\n", v.getResolvedValue().Type().String())
	return !v.getResolvedValue().IsValid()
}

// String returns a string for the underlying value. If this value is not
// of type string, pongo2 tries to convert it. Currently the following
// types for underlying values are supported:
//
//     1. string
//     2. int/uint (any size)
//     3. float (any precision)
//     4. bool
//     5. time.Time
//     6. String() will be called on the underlying value if provided
//
// NIL values will lead to an empty string. Unsupported types are leading
// to their respective type name.
func (v *Value) String() string {
	if v.IsNil() {
		return ""
	}

	switch v.getResolvedValue().Kind() {
	case reflect.String:
		return v.getResolvedValue().String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.getResolvedValue().Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.getResolvedValue().Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%f", v.getResolvedValue().Float())
	case reflect.Bool:
		if v.Bool() {
			return "True"
		}
		return "False"
	case reflect.Struct:
		if t, ok := v.Interface().(fmt.Stringer); ok {
			return t.String()
		}
	}

	logf("Value.String() not implemented for type: %s\n", v.getResolvedValue().Kind().String())
	return v.getResolvedValue().String()
}

// Integer returns the underlying value as an integer (converts the underlying
// value, if necessary). If it's not possible to convert the underlying value,
// it will return 0.
func (v *Value) Integer() int {
	switch v.getResolvedValue().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(v.getResolvedValue().Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int(v.getResolvedValue().Uint())
	case reflect.Float32, reflect.Float64:
		return int(v.getResolvedValue().Float())
	case reflect.String:
		// Try to convert from string to int (base 10)
		f, err := strconv.ParseFloat(v.getResolvedValue().String(), 64)
		if err != nil {
			return 0
		}
		return int(f)
	default:
		logf("Value.Integer() not available for type: %s\n", v.getResolvedValue().Kind().String())
		return 0
	}
}

// Float returns the underlying value as a float (converts the underlying
// value, if necessary). If it's not possible to convert the underlying value,
// it will return 0.0.
func (v *Value) Float() float64 {
	switch v.getResolvedValue().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.getResolvedValue().Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.getResolvedValue().Uint())
	case reflect.Float32, reflect.Float64:
		return v.getResolvedValue().Float()
	case reflect.String:
		// Try to convert from string to float64 (base 10)
		f, err := strconv.ParseFloat(v.getResolvedValue().String(), 64)
		if err != nil {
			return 0.0
		}
		return f
	default:
		logf("Value.Float() not available for type: %s\n", v.getResolvedValue().Kind().String())
		return 0.0
	}
}

// Bool returns the underlying value as bool. If the value is not bool, false
// will always be returned. If you're looking for true/false-evaluation of the
// underlying value, have a look on the IsTrue()-function.
func (v *Value) Bool() bool {
	switch v.getResolvedValue().Kind() {
	case reflect.Bool:
		return v.getResolvedValue().Bool()
	default:
		logf("Value.Bool() not available for type: %s\n", v.getResolvedValue().Kind().String())
		return false
	}
}

// IsTrue tries to evaluate the underlying value the Pythonic-way:
//
// Returns TRUE in one the following cases:
//
//     * int != 0
//     * uint != 0
//     * float != 0.0
//     * len(array/chan/map/slice/string) > 0
//     * bool == true
//     * underlying value is a struct
//
// Otherwise returns always FALSE.
func (v *Value) IsTrue() bool {
	switch v.getResolvedValue().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.getResolvedValue().Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.getResolvedValue().Uint() != 0
	case reflect.Float32, reflect.Float64:
		return v.getResolvedValue().Float() != 0
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return v.getResolvedValue().Len() > 0
	case reflect.Bool:
		return v.getResolvedValue().Bool()
	case reflect.Struct:
		return true // struct instance is always true
	default:
		logf("Value.IsTrue() not available for type: %s\n", v.getResolvedValue().Kind().String())
		return false
	}
}

// Negate tries to negate the underlying value. It's mainly used for
// the NOT-operator and in conjunction with a call to
// return_value.IsTrue() afterwards.
//
// Example:
//     AsValue(1).Negate().IsTrue() == false
func (v *Value) Negate() *Value {
	switch v.getResolvedValue().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.Integer() != 0 {
			return AsValue(0)
		}
		return AsValue(1)
	case reflect.Float32, reflect.Float64:
		if v.Float() != 0.0 {
			return AsValue(float64(0.0))
		}
		return AsValue(float64(1.1))
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return AsValue(v.getResolvedValue().Len() == 0)
	case reflect.Bool:
		return AsValue(!v.getResolvedValue().Bool())
	case reflect.Struct:
		return AsValue(false)
	default:
		logf("Value.IsTrue() not available for type: %s\n", v.getResolvedValue().Kind().String())
		return AsValue(true)
	}
}

// Len returns the length for an array, chan, map, slice or string.
// Otherwise it will return 0.
func (v *Value) Len() int {
	switch v.getResolvedValue().Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice:
		return v.getResolvedValue().Len()
	case reflect.String:
		runes := []rune(v.getResolvedValue().String())
		return len(runes)
	default:
		logf("Value.Len() not available for type: %s\n", v.getResolvedValue().Kind().String())
		return 0
	}
}

// Slice slices an array, slice or string. Otherwise it will
// return an empty []int.
func (v *Value) Slice(i, j int) *Value {
	switch v.getResolvedValue().Kind() {
	case reflect.Array, reflect.Slice:
		return AsValue(v.getResolvedValue().Slice(i, j).Interface())
	case reflect.String:
		runes := []rune(v.getResolvedValue().String())
		return AsValue(string(runes[i:j]))
	default:
		logf("Value.Slice() not available for type: %s\n", v.getResolvedValue().Kind().String())
		return AsValue([]int{})
	}
}

// Index gets the i-th item of an array, slice or string. Otherwise
// it will return NIL.
func (v *Value) Index(i int) *Value {
	switch v.getResolvedValue().Kind() {
	case reflect.Array, reflect.Slice:
		if i >= v.Len() {
			return AsValue(nil)
		}
		return AsValue(v.getResolvedValue().Index(i).Interface())
	case reflect.String:
		//return AsValue(v.getResolvedValue().Slice(i, i+1).Interface())
		s := v.getResolvedValue().String()
		runes := []rune(s)
		if i < len(runes) {
			return AsValue(string(runes[i]))
		}
		return AsValue("")
	default:
		logf("Value.Slice() not available for type: %s\n", v.getResolvedValue().Kind().String())
		return AsValue([]int{})
	}
}

// Contains checks whether the underlying value (which must be of type struct, map,
// string, array or slice) contains of another Value (e. g. used to check
// whether a struct contains of a specific field or a map contains a specific key).
//
// Example:
//     AsValue("Hello, World!").Contains(AsValue("World")) == true
func (v *Value) Contains(other *Value) bool {
	switch v.getResolvedValue().Kind() {
	case reflect.Struct:
		fieldValue := v.getResolvedValue().FieldByName(other.String())
		return fieldValue.IsValid()
	case reflect.Map:
		var mapValue reflect.Value
		switch other.Interface().(type) {
		case int:
			mapValue = v.getResolvedValue().MapIndex(other.getResolvedValue())
		case string:
			mapValue = v.getResolvedValue().MapIndex(other.getResolvedValue())
		default:
			logf("Value.Contains() does not support lookup type '%s'\n", other.getResolvedValue().Kind().String())
			return false
		}

		return mapValue.IsValid()
	case reflect.String:
		return strings.Contains(v.getResolvedValue().String(), other.String())

	case reflect.Slice, reflect.Array:
		for i := 0; i < v.getResolvedValue().Len(); i++ {
			item := v.getResolvedValue().Index(i)
			if other.Interface() == item.Interface() {
				return true
			}
		}
		return false

	default:
		logf("Value.Contains() not available for type: %s\n", v.getResolvedValue().Kind().String())
		return false
	}
}

// CanSlice checks whether the underlying value is of type array, slice or string.
// You normally would use CanSlice() before using the Slice() operation.
func (v *Value) CanSlice() bool {
	switch v.getResolvedValue().Kind() {
	case reflect.Array, reflect.Slice, reflect.String:
		return true
	}
	return false
}

// Iterate iterates over a map, array, slice or a string. It calls the
// function's first argument for every value with the following arguments:
//
//     idx      current 0-index
//     count    total amount of items
//     key      *Value for the key or item
//     value    *Value (only for maps, the respective value for a specific key)
//
// If the underlying value has no items or is not one of the types above,
// the empty function (function's second argument) will be called.
func (v *Value) Iterate(fn func(idx, count int, key, value *Value) bool, empty func()) {
	v.IterateOrder(fn, empty, false, false)
}

// IterateOrder behaves like Value.Iterate, but can iterate through an array/slice/string in reverse. Does
// not affect the iteration through a map because maps don't have any particular order.
// However, you can force an order using the `sorted` keyword (and even use `reversed sorted`).
func (v *Value) IterateOrder(fn func(idx, count int, key, value *Value) bool, empty func(), reverse bool, sorted bool) {
	switch v.getResolvedValue().Kind() {
	case reflect.Map:
		keys := sortedKeys(v.getResolvedValue().MapKeys())
		if sorted {
			if reverse {
				sort.Sort(sort.Reverse(keys))
			} else {
				sort.Sort(keys)
			}
		}
		keyLen := len(keys)
		for idx, key := range keys {
			value := v.getResolvedValue().MapIndex(key)
			if !fn(idx, keyLen, &Value{val: key}, &Value{val: value}) {
				return
			}
		}
		if keyLen == 0 {
			empty()
		}
		return // done
	case reflect.Array, reflect.Slice:
		var items valuesList

		itemCount := v.getResolvedValue().Len()
		for i := 0; i < itemCount; i++ {
			items = append(items, &Value{val: v.getResolvedValue().Index(i)})
		}

		if sorted {
			if reverse {
				sort.Sort(sort.Reverse(items))
			} else {
				sort.Sort(items)
			}
		} else {
			if reverse {
				for i := 0; i < itemCount/2; i++ {
					items[i], items[itemCount-1-i] = items[itemCount-1-i], items[i]
				}
			}
		}

		if len(items) > 0 {
			for idx, item := range items {
				if !fn(idx, itemCount, item, nil) {
					return
				}
			}
		} else {
			empty()
		}
		return // done
	case reflect.String:
		if sorted {
			// TODO(flosch): Handle sorted
			panic("TODO: handle sort for type string")
		}

		// TODO(flosch): Not utf8-compatible (utf8-decoding necessary)
		charCount := v.getResolvedValue().Len()
		if charCount > 0 {
			if reverse {
				for i := charCount - 1; i >= 0; i-- {
					if !fn(i, charCount, &Value{val: v.getResolvedValue().Slice(i, i+1)}, nil) {
						return
					}
				}
			} else {
				for i := 0; i < charCount; i++ {
					if !fn(i, charCount, &Value{val: v.getResolvedValue().Slice(i, i+1)}, nil) {
						return
					}
				}
			}
		} else {
			empty()
		}
		return // done
	default:
		logf("Value.Iterate() not available for type: %s\n", v.getResolvedValue().Kind().String())
	}
	empty()
}

// Interface gives you access to the underlying value.
func (v *Value) Interface() interface{} {
	if v.val.IsValid() {
		return v.val.Interface()
	}
	return nil
}

// EqualValueTo checks whether two values are containing the same value or object.
func (v *Value) EqualValueTo(other *Value) bool {
	// comparison of uint with int fails using .Interface()-comparison (see issue #64)
	if v.IsInteger() && other.IsInteger() {
		return v.Integer() == other.Integer()
	}
	return v.Interface() == other.Interface()
}

type sortedKeys []reflect.Value

func (sk sortedKeys) Len() int {
	return len(sk)
}

func (sk sortedKeys) Less(i, j int) bool {
	vi := &Value{val: sk[i]}
	vj := &Value{val: sk[j]}
	switch {
	case vi.IsInteger() && vj.IsInteger():
		return vi.Integer() < vj.Integer()
	case vi.IsFloat() && vj.IsFloat():
		return vi.Float() < vj.Float()
	default:
		return vi.String() < vj.String()
	}
}

func (sk sortedKeys) Swap(i, j int) {
	sk[i], sk[j] = sk[j], sk[i]
}

type valuesList []*Value

func (vl valuesList) Len() int {
	return len(vl)
}

func (vl valuesList) Less(i, j int) bool {
	vi := vl[i]
	vj := vl[j]
	switch {
	case vi.IsInteger() && vj.IsInteger():
		return vi.Integer() < vj.Integer()
	case vi.IsFloat() && vj.IsFloat():
		return vi.Float() < vj.Float()
	default:
		return vi.String() < vj.String()
	}
}

func (vl valuesList) Swap(i, j int) {
	vl[i], vl[j] = vl[j], vl[i]
}
