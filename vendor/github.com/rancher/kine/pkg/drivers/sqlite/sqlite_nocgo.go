// +build !cgo

package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/rancher/kine/pkg/drivers/generic"
	"github.com/rancher/kine/pkg/server"
)

var errNoCgo = errors.New("this binary is built without CGO, sqlite is disabled")

func New(ctx context.Context, dataSourceName string) (server.Backend, error) {
	return nil, errNoCgo
}

func NewVariant(driverName, dataSourceName string) (server.Backend, *generic.Generic, error) {
	return nil, nil, errNoCgo
}

func setup(db *sql.DB) error {
	return errNoCgo
}
