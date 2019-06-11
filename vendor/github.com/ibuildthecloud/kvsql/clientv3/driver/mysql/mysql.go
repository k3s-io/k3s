package mysql

import (
	"crypto/tls"
	"database/sql"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/ibuildthecloud/kvsql/clientv3/driver"
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
	createDB    = "create database if not exists kubernetes"
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

func Open(dataSourceName string, tlsConfig *tls.Config) (*sql.DB, error) {
	if dataSourceName == "" {
		dataSourceName = "root@unix(/var/run/mysqld/mysqld.sock)/"
	}
	// get database name
	dsList := strings.Split(dataSourceName, "/")
	databaseName := dsList[len(dsList)-1]
	if databaseName == "" {
		if err := createDBIfNotExist(dataSourceName); err != nil {
			return nil, err
		}
		dataSourceName = dataSourceName + "kubernetes"
	}

	// setting up tlsConfig
	if tlsConfig != nil {
		mysql.RegisterTLSConfig("custom", tlsConfig)
		if strings.Contains(dataSourceName, "?") {
			dataSourceName = dataSourceName + ",tls=custom"
		} else {
			dataSourceName = dataSourceName + "?tls=custom"
		}
	}

	db, err := sql.Open("mysql", dataSourceName)
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
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return err
	}
	_, err = db.Exec(createDB)
	if err != nil {
		return err
	}
	return nil
}

func createIndex(db *sql.DB, indexStmt string) error {
	_, err := db.Exec(indexStmt)
	if err != nil {
		// check if its a duplicate error
		if err.(*mysql.MySQLError).Number == 1061 {
			return nil
		}
		return err
	}
	return nil
}
