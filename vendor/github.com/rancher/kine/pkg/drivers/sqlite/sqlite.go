package sqlite

import (
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
		`CREATE TABLE IF NOT EXISTS key_value
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
		`CREATE INDEX IF NOT EXISTS key_value_name_index ON key_value (name)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS key_value_name_prev_revision_uindex ON key_value (name, prev_revision)`,
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
