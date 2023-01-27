package cri

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

const maxMsgSize = 1024 * 1024 * 16

// WaitForService blocks in a retry loop until the CRI service
// is functional at the provided socket address. It will return only on success,
// or when the context is cancelled.
func WaitForService(ctx context.Context, address string, service string) error {
	first := true
	for {
		conn, err := Connection(ctx, address)
		if err == nil {
			conn.Close()
			break
		}
		if first {
			first = false
		} else {
			logrus.Infof("Waiting for %s startup: %v", service, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	logrus.Infof("%s is now running", service)
	return nil
}
