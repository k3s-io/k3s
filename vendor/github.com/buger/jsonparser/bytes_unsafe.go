// +build !appengine,!appenginevm

package jsonparser

import (
	"reflect"
	"strconv"
	"unsafe"
)

//
// The reason for using *[]byte rather than []byte in parameters is an optimization. As of Go 1.6,
// the compiler cannot perfectly inline the function when using a non-pointer slice. That is,
// the non-pointer []byte parameter version is slower than if its function body is manually
// inlined, whereas the pointer []byte version is equally fast to the manually inlined
// version. Instruction count in assembly taken from "go tool compile" confirms this difference.
//
// TODO: Remove hack after Go 1.7 release
//
func equalStr(b *[]byte, s string) bool {
	return *(*string)(unsafe.Pointer(b)) == s
}

func parseFloat(b *[]byte) (float64, error) {
	return strconv.ParseFloat(*(*string)(unsafe.Pointer(b)), 64)
}

// A hack until issue golang/go#2632 is fixed.
// See: https://github.com/golang/go/issues/2632
func bytesToString(b *[]byte) string {
	return *(*string)(unsafe.Pointer(b))
}

func StringToBytes(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
}
