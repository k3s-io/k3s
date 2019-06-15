package mysql

import (
	"crypto/tls"
	"database/sql"
	"strings"

	"github.com/coreos/etcd/pkg/transport"
	"github.com/go-sql-driver/mysql"
	"github.com/ibuildthecloud/kvsql/clientv3/driver"
)

const (
	defaultUnixDSN = "root@unix(/var/run/mysqld/mysqld.sock)/"
	defaultHostDSN = "root@tcp(127.0.0.1)/"
)

var (
	fieldList = "name, value, old_value, old_revision, create_revision, revision, ttl, version, del"
	baseList  = `
SELECT kv.id, kv.name, kv.value, kv.old_value, kv.old_revision, kv.create_revision, kv.revision, kv.ttl, kv.version, kv.del
FROM key_value kv
  INNER JOIN
    (
      SELECT MAX(revision) revision, kvi.name
      FROM key_value kvi
		%REV%
        GROUP BY kvi.name
    ) AS r
    ON r.name = kv.name AND r.revision = kv.revision
WHERE kv.name like ? %RES% ORDER BY kv.name ASC limit ?
`
	insertSQL = `
INSERT INTO key_value(` + fieldList + `)
   VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	schema = []string{
		`create table if not exists key_value
			(
				name TEXT,
				value BLOB,
				create_revision INTEGER,
				revision INTEGER,
				ttl INTEGER,
				version INTEGER,
				del INTEGER,
				old_value BLOB,
				old_revision INTEGER,
				id INTEGER AUTO_INCREMENT,
				PRIMARY KEY (id)
			)`,
	}
	nameIdx     = "create index name_idx on key_value (name(100))"
	revisionIdx = "create index revision_idx on key_value (revision)"
	createDB    = "create database if not exists "
)

func NewMySQL() *driver.Generic {
	return &driver.Generic{
		CleanupSQL:      "DELETE FROM key_value WHERE ttl > 0 AND ttl < ?",
		GetSQL:          "SELECT id, " + fieldList + " FROM key_value WHERE name = ? ORDER BY revision DESC limit ?",
		ListSQL:         strings.Replace(strings.Replace(baseList, "%REV%", "", -1), "%RES%", "", -1),
		ListRevisionSQL: strings.Replace(strings.Replace(baseList, "%REV%", "WHERE kvi.revision >= ?", -1), "%RES%", "", -1),
		ListResumeSQL: strings.Replace(strings.Replace(baseList, "%REV%", "WHERE kvi.revision <= ?", -1),
			"%RES%", "and kv.name > ? ", -1),
		InsertSQL:      insertSQL,
		ReplaySQL:      "SELECT id, " + fieldList + " FROM key_value WHERE name like ? and revision >= ? ORDER BY revision ASC",
		GetRevisionSQL: "SELECT MAX(revision) FROM key_value",
		ToDeleteSQL:    "SELECT count(*), name, max(revision) FROM key_value GROUP BY name,del HAVING count(*) > 1 or (count(*)=1 and del=1)",
		DeleteOldSQL:   "DELETE FROM key_value WHERE name = ? AND (revision < ? OR (revision = ? AND del = 1))",
	}
}

func Open(dataSourceName string, tlsInfo *transport.TLSInfo) (*sql.DB, error) {
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, err
	}
	tlsConfig.MinVersion = tls.VersionTLS11
	if len(tlsInfo.CertFile) == 0 && len(tlsInfo.KeyFile) == 0 && len(tlsInfo.CAFile) == 0 {
		tlsConfig = nil
	}
	parsedDSN, err := prepareDSN(dataSourceName, tlsConfig)
	if err != nil {
		return nil, err
	}
	if err := createDBIfNotExist(parsedDSN); err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", parsedDSN)
	if err != nil {
		return nil, err
	}

	for _, stmt := range schema {
		_, err := db.Exec(stmt)
		if err != nil {
			return nil, err
		}
	}
	// check if duplicate indexes
	indexes := []string{
		nameIdx,
		revisionIdx}

	for _, idx := range indexes {
		err := createIndex(db, idx)
		if err != nil {
			return nil, err
		}
	}

	return db, nil
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

func createIndex(db *sql.DB, indexStmt string) error {
	_, err := db.Exec(indexStmt)
	if err != nil {
		if mysqlError, ok := err.(*mysql.MySQLError); !ok || mysqlError.Number != 1061 {
			return err
		}
	}
	return nil
}

func prepareDSN(dataSourceName string, tlsConfig *tls.Config) (string, error) {
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
		mysql.RegisterTLSConfig("custom", tlsConfig)
		config.TLSConfig = "custom"
	}
	dbName := "kubernetes"
	if len(config.DBName) > 0 {
		dbName = config.DBName
	}
	config.DBName = dbName
	parsedDSN := config.FormatDSN()

	return parsedDSN, nil
}
