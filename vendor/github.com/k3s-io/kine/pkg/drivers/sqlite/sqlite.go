// +build cgo

package sqlite

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/k3s-io/kine/pkg/drivers/generic"
	"github.com/k3s-io/kine/pkg/logstructured"
	"github.com/k3s-io/kine/pkg/logstructured/sqllog"
	"github.com/k3s-io/kine/pkg/server"
	"github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	// sqlite db driver
	_ "github.com/mattn/go-sqlite3"
)

var (
	schema = []string{
		`CREATE TABLE IF NOT EXISTS kine
			(
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name INTEGER,
				created INTEGER,
				deleted INTEGER,
				create_revision INTEGER,
				prev_revision INTEGER,
				lease INTEGER,
				value BLOB,
				old_value BLOB
			)`,
		`CREATE INDEX IF NOT EXISTS kine_name_index ON kine (name)`,
		`CREATE INDEX IF NOT EXISTS kine_name_id_index ON kine (name,id)`,
		`CREATE INDEX IF NOT EXISTS kine_id_deleted_index ON kine (id,deleted)`,
		`CREATE INDEX IF NOT EXISTS kine_prev_revision_index ON kine (prev_revision)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS kine_name_prev_revision_uindex ON kine (name, prev_revision)`,
	}
)

func New(ctx context.Context, dataSourceName string, connPoolConfig generic.ConnectionPoolConfig) (server.Backend, error) {
	backend, _, err := NewVariant(ctx, "sqlite3", dataSourceName, connPoolConfig)
	return backend, err
}

func NewVariant(ctx context.Context, driverName, dataSourceName string, connPoolConfig generic.ConnectionPoolConfig) (server.Backend, *generic.Generic, error) {
	if dataSourceName == "" {
		if err := os.MkdirAll("./db", 0700); err != nil {
			return nil, nil, err
		}
		dataSourceName = "./db/state.db?_journal=WAL&cache=shared"
	}

	dialect, err := generic.Open(ctx, driverName, dataSourceName, connPoolConfig, "?", false)
	if err != nil {
		return nil, nil, err
	}

	dialect.LastInsertID = true
	dialect.CompactSQL = `
		DELETE FROM kine AS kv
		WHERE
			kv.id IN (
				SELECT kp.prev_revision AS id
				FROM kine AS kp
				WHERE
					kp.name != 'compact_rev_key' AND
					kp.prev_revision != 0 AND
					kp.id <= ?
				UNION
				SELECT kd.id AS id
				FROM kine AS kd
				WHERE
					kd.deleted != 0 AND
					kd.id <= ?
			)`
	dialect.TranslateErr = func(err error) error {
		if err, ok := err.(sqlite3.Error); ok && err.ExtendedCode == sqlite3.ErrConstraintUnique {
			return server.ErrKeyExists
		}
		return err
	}

	// this is the first SQL that will be executed on a new DB conn so
	// loop on failure here because in the case of dqlite it could still be initializing
	for i := 0; i < 300; i++ {
		err = setup(dialect.DB)
		if err == nil {
			break
		}
		logrus.Errorf("failed to setup db: %v", err)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(time.Second):
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return nil, nil, errors.Wrap(err, "setup db")
	}

	dialect.Migrate(context.Background())
	return logstructured.New(sqllog.New(dialect)), dialect, nil
}

func setup(db *sql.DB) error {
	logrus.Infof("Configuring database table schema and indexes, this may take a moment...")

	for _, stmt := range schema {
		logrus.Tracef("SETUP EXEC : %v", generic.Stripped(stmt))
		_, err := db.Exec(stmt)
		if err != nil {
			return err
		}
	}

	logrus.Infof("Database tables and indexes are up to date")
	return nil
}
