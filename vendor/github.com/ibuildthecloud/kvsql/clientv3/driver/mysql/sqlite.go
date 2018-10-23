package sqlite

import (
	"database/sql"
	"github.com/ibuildthecloud/kvsql/clientv3/driver"
	"github.com/ibuildthecloud/kvsql/clientv3/driver/sqlite"
)

var (
	schema = []string{
		`create table if not exists key_value
			(
				name int not null,
				value MEDIUMTEXT not null,
				create_revision int not null,
				revision int not null,
				ttl int not null,
				version int not null,
				del int not null,
				old_value MEDIUMTEXT not null,
				id int auto_increment,
				old_revision int not null,
				constraint key_value_pk
				primary key (id)
			)`,
	}

	idx = []string{
		"create index key_value__name_idx on key_value (name)",
		"create index key_value__revision_idx on key_value (revision)",
	}
)

func NewMYSQL() *driver.Generic {
	return sqlite.NewSQLite()
}

func Open(dataSourceName string) (*sql.DB, error) {
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

	for _, stmt := range idx {
		db.Exec(stmt)
	}

	return db, nil
}
