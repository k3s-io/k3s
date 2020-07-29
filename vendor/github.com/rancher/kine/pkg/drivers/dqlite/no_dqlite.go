// +build !dqlite

package dqlite

import (
	"context"
	"fmt"

	"github.com/rancher/kine/pkg/drivers/generic"
	"github.com/rancher/kine/pkg/server"
)

func New(ctx context.Context, datasourceName string, connPoolConfig generic.ConnectionPoolConfig) (server.Backend, error) {
	return nil, fmt.Errorf("dqlite is not support, compile with \"-tags dqlite\"")
}
