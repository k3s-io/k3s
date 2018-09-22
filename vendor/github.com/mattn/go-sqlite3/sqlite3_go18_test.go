// Copyright (C) 2014 Yasuhiro Matsumoto <mattn.jp@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// +build go1.8

package sqlite3

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestNamedParams(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec(`
	create table foo (id integer, name text, extra text);
	`)
	if err != nil {
		t.Error("Failed to call db.Query:", err)
	}

	_, err = db.Exec(`insert into foo(id, name, extra) values(:id, :name, :name)`, sql.Named("name", "foo"), sql.Named("id", 1))
	if err != nil {
		t.Error("Failed to call db.Exec:", err)
	}

	row := db.QueryRow(`select id, extra from foo where id = :id and extra = :extra`, sql.Named("id", 1), sql.Named("extra", "foo"))
	if row == nil {
		t.Error("Failed to call db.QueryRow")
	}
	var id int
	var extra string
	err = row.Scan(&id, &extra)
	if err != nil {
		t.Error("Failed to db.Scan:", err)
	}
	if id != 1 || extra != "foo" {
		t.Error("Failed to db.QueryRow: not matched results")
	}
}

var (
	testTableStatements = []string{
		`DROP TABLE IF EXISTS test_table`,
		`
CREATE TABLE IF NOT EXISTS test_table (
	key1      VARCHAR(64) PRIMARY KEY,
	key_id    VARCHAR(64) NOT NULL,
	key2      VARCHAR(64) NOT NULL,
	key3      VARCHAR(64) NOT NULL,
	key4      VARCHAR(64) NOT NULL,
	key5      VARCHAR(64) NOT NULL,
	key6      VARCHAR(64) NOT NULL,
	data      BLOB        NOT NULL
);`,
	}
	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func initDatabase(t *testing.T, db *sql.DB, rowCount int64) {
	for _, query := range testTableStatements {
		_, err := db.Exec(query)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := int64(0); i < rowCount; i++ {
		query := `INSERT INTO test_table
			(key1, key_id, key2, key3, key4, key5, key6, data)
			VALUES
			(?, ?, ?, ?, ?, ?, ?, ?);`
		args := []interface{}{
			randStringBytes(50),
			fmt.Sprint(i),
			randStringBytes(50),
			randStringBytes(50),
			randStringBytes(50),
			randStringBytes(50),
			randStringBytes(50),
			randStringBytes(50),
			randStringBytes(2048),
		}
		_, err := db.Exec(query, args...)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestShortTimeout(t *testing.T) {
	srcTempFilename := TempFilename(t)
	defer os.Remove(srcTempFilename)

	db, err := sql.Open("sqlite3", srcTempFilename)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	initDatabase(t, db, 100)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancel()
	query := `SELECT key1, key_id, key2, key3, key4, key5, key6, data
		FROM test_table
		ORDER BY key2 ASC`
	_, err = db.QueryContext(ctx, query)
	if err != nil && err != context.DeadlineExceeded {
		t.Fatal(err)
	}
	if ctx.Err() != nil && ctx.Err() != context.DeadlineExceeded {
		t.Fatal(ctx.Err())
	}
}

func TestExecCancel(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err = db.Exec("create table foo (id integer primary key)"); err != nil {
		t.Fatal(err)
	}

	for n := 0; n < 100; n++ {
		ctx, cancel := context.WithCancel(context.Background())
		_, err = db.ExecContext(ctx, "insert into foo (id) values (?)", n)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}
}
