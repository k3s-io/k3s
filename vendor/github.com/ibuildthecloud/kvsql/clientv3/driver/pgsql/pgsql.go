package pgsql

import (
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"github.com/ibuildthecloud/kvsql/clientv3/driver"
	"github.com/lib/pq"
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
 				value bytea,
 				create_revision INTEGER,
 				revision INTEGER,
 				ttl INTEGER,
 				version INTEGER,
 				del INTEGER,
 				old_value bytea,
 				id SERIAL PRIMARY KEY,
 				old_revision INTEGER
 			);`,
		`create index if not exists name_idx on key_value (name)`,
		`create index if not exists revision_idx on key_value (revision)`,
	}
	createDB = "create database kubernetes"
)

func NewPGSQL() *driver.Generic {
	return &driver.Generic{
		CleanupSQL:      q("DELETE FROM key_value WHERE ttl > 0 AND ttl < ?"),
		GetSQL:          q("SELECT id, " + fieldList + " FROM key_value WHERE name=? ORDER BY revision DESC limit ?"),
		ListSQL:         q(strings.Replace(strings.Replace(baseList, "%REV%", "", -1), "%RES%", "", -1)),
		ListRevisionSQL: q(strings.Replace(strings.Replace(baseList, "%REV%", "WHERE kvi.revision>=?", -1), "%RES%", "", -1)),
		ListResumeSQL: q(strings.Replace(strings.Replace(baseList, "%REV%", "WHERE kvi.revision<=?", -1),
			"%RES%", "and kv.name > ? ", -1)),
		InsertSQL:      q(insertSQL),
		ReplaySQL:      q("SELECT id, " + fieldList + " FROM key_value WHERE name like ? and revision>=? ORDER BY revision ASC"),
		GetRevisionSQL: q("SELECT MAX(revision) FROM key_value"),
		ToDeleteSQL:    q("SELECT count(*), name, max(revision) FROM key_value GROUP BY name,del HAVING count(*) > 1 or (count(*)=1 and del=1)"),
		DeleteOldSQL:   q("DELETE FROM key_value WHERE name=? AND (revision<? OR (revision=? AND del=1))"),
	}
}

func Open(dataSourceName string) (*sql.DB, error) {
	if dataSourceName == "" {
		dataSourceName = "postgres://postgres:postgres@localhost/"
	} else {
		dataSourceName = "postgres://" + dataSourceName
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
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return nil, err
	}

	for _, stmt := range schema {
		_, err := db.Exec(stmt)
		if err != nil {
			return nil, err
		}
	}

	return db, nil
}

func createDBIfNotExist(dataSourceName string) error {
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return err
	}
	_, err = db.Exec(createDB)
	// check if database already exists
	if err != nil && err.(*pq.Error).Code != "42P04" {
		return err
	}
	return nil
}

func q(sql string) string {
	regex := regexp.MustCompile(`\?`)
	pref := "$"
	n := 0
	return regex.ReplaceAllStringFunc(sql, func(string) string {
		n++
		return pref + strconv.Itoa(n)
	})
}
