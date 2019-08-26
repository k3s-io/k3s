package sqlite

import (
	"context"
	"database/sql"
	"os"

	"github.com/rancher/kine/pkg/drivers/generic"
	"github.com/rancher/kine/pkg/logstructured"
	"github.com/rancher/kine/pkg/logstructured/sqllog"
	"github.com/rancher/kine/pkg/server"

	// sqlite db driver
	_ "github.com/mattn/go-sqlite3"
)

var (
	schema = []string{
		`CREATE TABLE IF NOT EXISTS kine
			(
				id INTEGER primary key autoincrement,
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
		`CREATE UNIQUE INDEX IF NOT EXISTS kine_name_prev_revision_uindex ON kine (name, prev_revision)`,
	}
)

func New(dataSourceName string) (server.Backend, error) {
	if dataSourceName == "" {
		if err := os.MkdirAll("./db", 0700); err != nil {
			return nil, err
		}
		dataSourceName = "./db/state.db?_journal=WAL&cache=shared"
	}

	dialect, err := generic.Open("sqlite3", dataSourceName, "?", false)
	if err != nil {
		return nil, err
	}
	dialect.LastInsertID = true

	if err := setup(dialect.DB); err != nil {
		return nil, err
	}

	dialect.Migrate(context.Background())
	return logstructured.New(sqllog.New(dialect)), nil
}

func setup(db *sql.DB) error {
	for _, stmt := range schema {
		_, err := db.Exec(stmt)
		if err != nil {
			return err
		}
	}

	return nil
}
