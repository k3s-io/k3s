package goStrongswanVici

import (
	"fmt"
)

func handlePanic(f func() error) (err error) {
	defer func() {
		r := recover()
		//no panic
		if r == nil {
			return
		}
		//panic a error
		if e, ok := r.(error); ok {
			err = e
			return
		}
		//panic another stuff
		err = fmt.Errorf("%s", r)
	}()
	err = f()
	return
}
