//go:build linux
// +build linux

package cri

import (
	"context"
	"time"

	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	k8sutil "k8s.io/cri-client/pkg/util"
)

const socketPrefix = "unix://"

// Connection connects to a CRI socket at the given path.
func Connection(ctx context.Context, address string) (*grpc.ClientConn, error) {
	addr, dialer, err := k8sutil.GetAddressAndDialer(socketPrefix + address)
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
