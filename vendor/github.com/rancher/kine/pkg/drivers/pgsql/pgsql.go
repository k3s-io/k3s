package pgsql

import (
	"database/sql"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/coreos/etcd/pkg/transport"
	"github.com/lib/pq"
	"github.com/rancher/kine/pkg/drivers/generic"
	"github.com/rancher/kine/pkg/logstructured"
	"github.com/rancher/kine/pkg/logstructured/sqllog"
	"github.com/rancher/kine/pkg/server"
)

const (
	defaultDSN = "postgres://postgres:postgres@localhost/"
)

var (
	schema = []string{
		`create table if not exists key_value
 			(
 				id SERIAL PRIMARY KEY,
 				name TEXT,
				created INTEGER,
				deleted INTEGER,
 				create_revision INTEGER,
 				prev_revision INTEGER,
 				lease INTEGER,
 				value bytea,
 				old_value bytea
 			);`,
		`CREATE INDEX IF NOT EXISTS key_value_name_index ON key_value (name)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS key_value_name_prev_revision_uindex ON key_value (name, prev_revision)`,
	}
	createDB = "create database "
)

func New(dataSourceName string, tlsInfo *transport.TLSInfo) (server.Backend, error) {
	parsedDSN, err := prepareDSN(dataSourceName, tlsInfo)
	if err != nil {
		return nil, err
	}

	if err := createDBIfNotExist(parsedDSN); err != nil {
		return nil, err
	}

	dialect, err := generic.Open("postgres", parsedDSN, "$", true)
	if err != nil {
		return nil, err
	}

	if err := setup(dialect.DB, parsedDSN, tlsInfo); err != nil {
		return nil, err
	}

	return logstructured.New(sqllog.New(dialect)), nil
}

func setup(db *sql.DB, parsedDSN string, tlsInfo *transport.TLSInfo) error {
	for _, stmt := range schema {
		_, err := db.Exec(stmt)
		if err != nil {
			return err
		}
	}

	return nil
}

func createDBIfNotExist(dataSourceName string) error {
	u, err := url.Parse(dataSourceName)
	if err != nil {
		return err
	}

	dbName := strings.SplitN(u.Path, "/", 2)[1]
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return err
	}
	defer db.Close()

	err = db.Ping()
	// check if database already exists
	if _, ok := err.(*pq.Error); !ok {
		return err
	}
	if err := err.(*pq.Error); err.Code != "42P04" {
		if err.Code != "3D000" {
			return err
		}
		// database doesn't exit, will try to create it
		u.Path = "/postgres"
		db, err := sql.Open("postgres", u.String())
		if err != nil {
			return err
		}
		defer db.Close()
		_, err = db.Exec(createDB + dbName + ";")
		if err != nil {
			return err
		}
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

func prepareDSN(dataSourceName string, tlsInfo *transport.TLSInfo) (string, error) {
	if len(dataSourceName) == 0 {
		dataSourceName = defaultDSN
	} else {
		dataSourceName = "postgres://" + dataSourceName
	}
	u, err := url.Parse(dataSourceName)
	if err != nil {
		return "", err
	}
	if len(u.Path) == 0 || u.Path == "/" {
		u.Path = "/kubernetes"
	}

	queryMap, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", err
	}
	// set up tls dsn
	params := url.Values{}
	sslmode := ""
	if _, ok := queryMap["sslcert"]; tlsInfo.CertFile != "" && !ok {
		params.Add("sslcert", tlsInfo.CertFile)
		sslmode = "verify-full"
	}
	if _, ok := queryMap["sslkey"]; tlsInfo.KeyFile != "" && !ok {
		params.Add("sslkey", tlsInfo.KeyFile)
		sslmode = "verify-full"
	}
	if _, ok := queryMap["sslrootcert"]; tlsInfo.CAFile != "" && !ok {
		params.Add("sslrootcert", tlsInfo.CAFile)
		sslmode = "verify-full"
	}
	if _, ok := queryMap["sslmode"]; !ok && sslmode != "" {
		params.Add("sslmode", sslmode)
	}
	for k, v := range queryMap {
		params.Add(k, v[0])
	}
	u.RawQuery = params.Encode()
	return u.String(), nil
}
