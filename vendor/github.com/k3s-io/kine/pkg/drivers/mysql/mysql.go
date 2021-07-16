package mysql

import (
	"context"
	cryptotls "crypto/tls"
	"database/sql"

	"github.com/go-sql-driver/mysql"
	"github.com/k3s-io/kine/pkg/drivers/generic"
	"github.com/k3s-io/kine/pkg/logstructured"
	"github.com/k3s-io/kine/pkg/logstructured/sqllog"
	"github.com/k3s-io/kine/pkg/server"
	"github.com/k3s-io/kine/pkg/tls"
	"github.com/sirupsen/logrus"
)

const (
	defaultUnixDSN = "root@unix(/var/run/mysqld/mysqld.sock)/"
	defaultHostDSN = "root@tcp(127.0.0.1)/"
)

var (
	schema = []string{
		`CREATE TABLE IF NOT EXISTS kine
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
		`CREATE INDEX kine_name_index ON kine (name)`,
		`CREATE INDEX kine_name_id_index ON kine (name,id)`,
		`CREATE INDEX kine_id_deleted_index ON kine (id,deleted)`,
		`CREATE INDEX kine_prev_revision_index ON kine (prev_revision)`,
		`CREATE UNIQUE INDEX kine_name_prev_revision_uindex ON kine (name, prev_revision)`,
	}
	createDB = "CREATE DATABASE IF NOT EXISTS "
)

func New(ctx context.Context, dataSourceName string, tlsInfo tls.Config, connPoolConfig generic.ConnectionPoolConfig) (server.Backend, error) {
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

	dialect, err := generic.Open(ctx, "mysql", parsedDSN, connPoolConfig, "?", false)
	if err != nil {
		return nil, err
	}

	dialect.LastInsertID = true
	dialect.CompactSQL = `
		DELETE kv FROM kine AS kv
		INNER JOIN (
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
		) AS ks
		ON kv.id = ks.id`
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
	logrus.Infof("Configuring database table schema and indexes, this may take a moment...")

	for _, stmt := range schema {
		logrus.Tracef("SETUP EXEC : %v", generic.Stripped(stmt))
		_, err := db.Exec(stmt)
		if err != nil {
			if mysqlError, ok := err.(*mysql.MySQLError); !ok || mysqlError.Number != 1061 {
				return err
			}
		}
	}

	logrus.Infof("Database tables and indexes are up to date")
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
