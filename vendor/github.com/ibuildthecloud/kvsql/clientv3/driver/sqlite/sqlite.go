package sqlite

import (
	"database/sql"
	"strings"

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
				name INTEGER,
				value BLOB,
				create_revision INTEGER,
				revision INTEGER,
				ttl INTEGER,
				version INTEGER,
				del INTEGER,
				old_value BLOB,
				id INTEGER primary key autoincrement,
				old_revision INTEGER
			)`,
		`create index if not exists name_idx on key_value (name)`,
		`create index if not exists revision_idx on key_value (revision)`,
	}
)

func NewSQLite() *driver.Generic {
	return &driver.Generic{
		CleanupSQL:      "DELETE FROM key_value WHERE ttl > 0 AND ttl < ?",
		GetSQL:          "SELECT id, " + fieldList + " FROM key_value WHERE name = ? ORDER BY revision DESC limit ?",
		ListSQL:         strings.Replace(strings.Replace(baseList, "%REV%", "", -1), "%RES%", "", -1),
		ListRevisionSQL: strings.Replace(strings.Replace(baseList, "%REV%", "WHERE kvi.revision >= ?", -1), "%RES%", "", -1),
		ListResumeSQL: strings.Replace(strings.Replace(baseList, "%REV%", "WHERE kvi.revision <= ?", -1),
			"%RES%", "and kv.name > ? ", -1),
		InsertSQL:      insertSQL,
		ReplaySQL:      "SELECT id, " + fieldList + " FROM key_value WHERE name like ? and revision > ? ORDER BY revision ASC",
		GetRevisionSQL: "SELECT MAX(revision) FROM key_value",
		ToDeleteSQL:    "SELECT count(*) c, name, max(revision) FROM key_value GROUP BY name HAVING c > 1 or (c = 1 and del = 1)",
		DeleteOldSQL:   "DELETE FROM key_value WHERE name = ? AND (revision < ? OR (revision = ? AND del = 1))",
	}
}

func Open(dataSourceName string) (*sql.DB, error) {
	if dataSourceName == "" {
		dataSourceName = "./state.db?_journal=WAL&cache=shared"
	}
	db, err := sql.Open("sqlite3", dataSourceName)
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
