// +build !cgo

package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/k3s-io/kine/pkg/drivers/generic"
	"github.com/k3s-io/kine/pkg/server"
)

var errNoCgo = errors.New("this binary is built without CGO, sqlite is disabled")

func New(ctx context.Context, dataSourceName string, connPoolConfig generic.ConnectionPoolConfig) (server.Backend, error) {
	return nil, errNoCgo
}

func NewVariant(driverName, dataSourceName string, connPoolConfig generic.ConnectionPoolConfig) (server.Backend, *generic.Generic, error) {
	return nil, nil, errNoCgo
}

func setup(db *sql.DB) error {
	return errNoCgo
}
