// Copyright (c) 2017 Gorillalabs. All rights reserved.

package backend

import "io"

type Waiter interface {
	Wait() error
}

type Starter interface {
	StartProcess(cmd string, args ...string) (Waiter, io.Writer, io.Reader, io.Reader, error)
}
