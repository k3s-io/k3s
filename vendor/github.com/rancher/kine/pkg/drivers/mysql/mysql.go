package mysql

import (
	"context"
	cryptotls "crypto/tls"
	"database/sql"

	"github.com/go-sql-driver/mysql"
	"github.com/rancher/kine/pkg/drivers/generic"
	"github.com/rancher/kine/pkg/logstructured"
	"github.com/rancher/kine/pkg/logstructured/sqllog"
	"github.com/rancher/kine/pkg/server"
	"github.com/rancher/kine/pkg/tls"
)

const (
	defaultUnixDSN = "root@unix(/var/run/mysqld/mysqld.sock)/"
	defaultHostDSN = "root@tcp(127.0.0.1)/"
)

var (
	schema = []string{
		`create table if not exists kine
			(
				id INTEGER AUTO_INCREMENT,
				name VARCHAR(630),
				created INTEGER,
				deleted INTEGER,
				create_revision INTEGER,
 				prev_revision INTEGER,
				lease INTEGER,
				value MEDIUMBLOB,
				old_value MEDIUMBLOB,
				PRIMARY KEY (id)
			);`,
	}
	nameIdx     = "create index kine_name_index on kine (name)"
	nameIDIdx   = "create index kine_name_id_index on kine (name,id)"
	revisionIdx = "create unique index kine_name_prev_revision_uindex on kine (name, prev_revision)"
	createDB    = "create database if not exists "
)

func New(ctx context.Context, dataSourceName string, tlsInfo tls.Config) (server.Backend, error) {
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, err
	}

	if tlsConfig != nil {
		tlsConfig.MinVersion = cryptotls.VersionTLS11
	}

	parsedDSN, err := prepareDSN(dataSourceName, tlsConfig)
	if err != nil {
		return nil, err
	}

	if err := createDBIfNotExist(parsedDSN); err != nil {
		return nil, err
	}

	dialect, err := generic.Open(ctx, "mysql", parsedDSN, "?", false)
	if err != nil {
		return nil, err
	}
	dialect.LastInsertID = true
	dialect.TranslateErr = func(err error) error {
		if err, ok := err.(*mysql.MySQLError); ok && err.Number == 1062 {
			return server.ErrKeyExists
		}
		return err
	}
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
	// check if duplicate indexes
	indexes := []string{
		nameIdx,
		nameIDIdx,
		revisionIdx}

	for _, idx := range indexes {
		err := createIndex(db, idx)
		if err != nil {
			return err
		}
	}
	return nil
}

func createDBIfNotExist(dataSourceName string) error {
	config, err := mysql.ParseDSN(dataSourceName)
	if err != nil {
		return err
	}
	dbName := config.DBName

	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return err
	}
	_, err = db.Exec(createDB + dbName)
	if err != nil {
		if mysqlError, ok := err.(*mysql.MySQLError); !ok || mysqlError.Number != 1049 {
			return err
		}
		config.DBName = ""
		db, err = sql.Open("mysql", config.FormatDSN())
		if err != nil {
			return err
		}
		_, err = db.Exec(createDB + dbName)
		if err != nil {
			return err
		}
	}
	return nil
}

func prepareDSN(dataSourceName string, tlsConfig *cryptotls.Config) (string, error) {
	if len(dataSourceName) == 0 {
		dataSourceName = defaultUnixDSN
		if tlsConfig != nil {
			dataSourceName = defaultHostDSN
		}
	}
	config, err := mysql.ParseDSN(dataSourceName)
	if err != nil {
		return "", err
	}
	// setting up tlsConfig
	if tlsConfig != nil {
		if err := mysql.RegisterTLSConfig("kine", tlsConfig); err != nil {
			return "", err
		}
		config.TLSConfig = "kine"
	}
	dbName := "kubernetes"
	if len(config.DBName) > 0 {
		dbName = config.DBName
	}
	config.DBName = dbName
	parsedDSN := config.FormatDSN()

	return parsedDSN, nil
}

func createIndex(db *sql.DB, indexStmt string) error {
	_, err := db.Exec(indexStmt)
	if err != nil {
		if mysqlError, ok := err.(*mysql.MySQLError); !ok || mysqlError.Number != 1061 {
			return err
		}
	}
	return nil
}
