package cri

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	k8sutil "k8s.io/cri-client/pkg/util"
)

const maxMsgSize = 1024 * 1024 * 16

// Connection connects to a CRI socket at the given path.
func Connection(ctx context.Context, address string) (*grpc.ClientConn, error) {
	if !strings.HasPrefix(address, socketPrefix) {
		address = socketPrefix + address
	}

	addr, dialer, err := k8sutil.GetAddressAndDialer(address)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(3*time.Second), grpc.WithContextDialer(dialer), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)))
	if err != nil {
		return nil, err
	}

	c := runtimeapi.NewRuntimeServiceClient(conn)
	_, err = c.Version(ctx, &runtimeapi.VersionRequest{
		Version: "0.1.0",
	})
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

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
