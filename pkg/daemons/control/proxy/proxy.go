package proxy

import (
	"errors"
	"io"
	"net"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
)

// Proxy copies data in both directions between lconn and rconn. When either
// direction ends, both connections are closed so the other direction unblocks,
// and Proxy returns only after both copy goroutines have exited.
func Proxy(lconn, rconn io.ReadWriteCloser) error {
	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	closeBoth := func(err error) {
		once.Do(func() {
			firstErr = err
			lconn.Close()
			rconn.Close()
		})
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := io.Copy(rconn, lconn)
		closeBoth(err)
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(lconn, rconn)
		closeBoth(err)
	}()
	wg.Wait()

	if firstErr != nil {
		if isConnTeardown(firstErr) {
			logrus.Debugf("Proxy connection closed: %v", firstErr)
		} else {
			logrus.Warnf("Proxy error: %v", firstErr)
		}
	}
	return firstErr
}

// isConnTeardown reports errors that are expected when one side closes the connection first.
func isConnTeardown(err error) bool {
	return errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE)
}
