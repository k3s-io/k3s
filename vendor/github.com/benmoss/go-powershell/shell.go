// Copyright (c) 2017 Gorillalabs. All rights reserved.

package powershell

import (
	"fmt"
	"io"
	"regexp"
	"sync"

	"github.com/k3s-io/go-powershell/backend"
	"github.com/k3s-io/go-powershell/utils"
	"github.com/pkg/errors"
)

const newline = "\r\n"

type Shell interface {
	Execute(cmd string) (string, string, error)
	Exit()
}

type shell struct {
	handle backend.Waiter
	stdin  io.Writer
	stdout io.Reader
	stderr io.Reader
}

func New(backend backend.Starter) (Shell, error) {
	handle, stdin, stdout, stderr, err := backend.StartProcess("powershell.exe", "-NoExit", "-Command", "-")
	if err != nil {
		return nil, err
	}

	return &shell{handle, stdin, stdout, stderr}, nil
}

func (s *shell) Execute(cmd string) (string, string, error) {
	if s.handle == nil {
		return "", "", errors.Wrap(errors.New(cmd), "Cannot execute commands on closed shells.")
	}

	outBoundary := createBoundary()
	errBoundary := createBoundary()

	// wrap the command in special markers so we know when to stop reading from the pipes
	full := fmt.Sprintf("%s; echo '%s'; [Console]::Error.WriteLine('%s')%s", cmd, outBoundary, errBoundary, newline)

	_, err := s.stdin.Write([]byte(full))
	if err != nil {
		return "", "", errors.Wrap(errors.Wrap(err, cmd), "Could not send PowerShell command")
	}

	// read stdout and stderr
	sout := ""
	serr := ""

	waiter := &sync.WaitGroup{}
	waiter.Add(2)

	go streamReader(s.stdout, outBoundary, &sout, waiter)
	go streamReader(s.stderr, errBoundary, &serr, waiter)

	waiter.Wait()

	if len(serr) > 0 {
		return sout, serr, errors.Wrap(errors.New(cmd), serr)
	}

	return sout, serr, nil
}

func (s *shell) Exit() {
	s.stdin.Write([]byte("exit" + newline))

	// if it's possible to close stdin, do so (some backends, like the local one,
	// do support it)
	closer, ok := s.stdin.(io.Closer)
	if ok {
		closer.Close()
	}

	s.handle.Wait()

	s.handle = nil
	s.stdin = nil
	s.stdout = nil
	s.stderr = nil
}

func streamReader(stream io.Reader, boundary string, buffer *string, signal *sync.WaitGroup) error {
	// read all output until we have found our boundary token
	output := ""
	bufsize := 64
	marker := regexp.MustCompile("(?s)(.*)" + regexp.QuoteMeta(boundary))

	for {
		buf := make([]byte, bufsize)
		read, err := stream.Read(buf)
		if err != nil {
			return err
		}

		output = output + string(buf[:read])

		if marker.MatchString(output) {
			break
		}
	}

	*buffer = marker.FindStringSubmatch(output)[1]
	signal.Done()

	return nil
}

func createBoundary() string {
	return "$gorilla" + utils.CreateRandomString(12) + "$"
}
