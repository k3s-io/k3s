// Copyright (C) 2014 Yasuhiro Matsumoto <mattn.jp@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package sqlite3

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TempFilename(t *testing.T) string {
	f, err := ioutil.TempFile("", "go-sqlite3-test-")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func doTestOpen(t *testing.T, option string) (string, error) {
	var url string
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	if option != "" {
		url = tempFilename + option
	} else {
		url = tempFilename
	}
	db, err := sql.Open("sqlite3", url)
	if err != nil {
		return "Failed to open database:", err
	}
	defer os.Remove(tempFilename)
	defer db.Close()

	_, err = db.Exec("drop table foo")
	_, err = db.Exec("create table foo (id integer)")
	if err != nil {
		return "Failed to create table:", err
	}

	if stat, err := os.Stat(tempFilename); err != nil || stat.IsDir() {
		return "Failed to create ./foo.db", nil
	}

	return "", nil
}

func TestOpen(t *testing.T) {
	cases := map[string]bool{
		"":                   true,
		"?_txlock=immediate": true,
		"?_txlock=deferred":  true,
		"?_txlock=exclusive": true,
		"?_txlock=bogus":     false,
	}
	for option, expectedPass := range cases {
		result, err := doTestOpen(t, option)
		if result == "" {
			if !expectedPass {
				errmsg := fmt.Sprintf("_txlock error not caught at dbOpen with option: %s", option)
				t.Fatal(errmsg)
			}
		} else if expectedPass {
			if err == nil {
				t.Fatal(result)
			} else {
				t.Fatal(result, err)
			}
		}
	}
}

func TestReadonly(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)

	db1, err := sql.Open("sqlite3", "file:"+tempFilename)
	if err != nil {
		t.Fatal(err)
	}
	db1.Exec("CREATE TABLE test (x int, y float)")

	db2, err := sql.Open("sqlite3", "file:"+tempFilename+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	_ = db2
	_, err = db2.Exec("INSERT INTO test VALUES (1, 3.14)")
	if err == nil {
		t.Fatal("didn't expect INSERT into read-only database to work")
	}
}

func TestForeignKeys(t *testing.T) {
	cases := map[string]bool{
		"?_foreign_keys=1": true,
		"?_foreign_keys=0": false,
	}
	for option, want := range cases {
		fname := TempFilename(t)
		uri := "file:" + fname + option
		db, err := sql.Open("sqlite3", uri)
		if err != nil {
			os.Remove(fname)
			t.Errorf("sql.Open(\"sqlite3\", %q): %v", uri, err)
			continue
		}
		var enabled bool
		err = db.QueryRow("PRAGMA foreign_keys;").Scan(&enabled)
		db.Close()
		os.Remove(fname)
		if err != nil {
			t.Errorf("query foreign_keys for %s: %v", uri, err)
			continue
		}
		if enabled != want {
			t.Errorf("\"PRAGMA foreign_keys;\" for %q = %t; want %t", uri, enabled, want)
			continue
		}
	}
}

func TestRecursiveTriggers(t *testing.T) {
	cases := map[string]bool{
		"?_recursive_triggers=1": true,
		"?_recursive_triggers=0": false,
	}
	for option, want := range cases {
		fname := TempFilename(t)
		uri := "file:" + fname + option
		db, err := sql.Open("sqlite3", uri)
		if err != nil {
			os.Remove(fname)
			t.Errorf("sql.Open(\"sqlite3\", %q): %v", uri, err)
			continue
		}
		var enabled bool
		err = db.QueryRow("PRAGMA recursive_triggers;").Scan(&enabled)
		db.Close()
		os.Remove(fname)
		if err != nil {
			t.Errorf("query recursive_triggers for %s: %v", uri, err)
			continue
		}
		if enabled != want {
			t.Errorf("\"PRAGMA recursive_triggers;\" for %q = %t; want %t", uri, enabled, want)
			continue
		}
	}
}

func TestClose(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}

	_, err = db.Exec("drop table foo")
	_, err = db.Exec("create table foo (id integer)")
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	stmt, err := db.Prepare("select id from foo where id = ?")
	if err != nil {
		t.Fatal("Failed to select records:", err)
	}

	db.Close()
	_, err = stmt.Exec(1)
	if err == nil {
		t.Fatal("Failed to operate closed statement")
	}
}

func TestInsert(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec("drop table foo")
	_, err = db.Exec("create table foo (id integer)")
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	res, err := db.Exec("insert into foo(id) values(123)")
	if err != nil {
		t.Fatal("Failed to insert record:", err)
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		t.Fatalf("Expected %d for affected rows, but %d:", 1, affected)
	}

	rows, err := db.Query("select id from foo")
	if err != nil {
		t.Fatal("Failed to select records:", err)
	}
	defer rows.Close()

	rows.Next()

	var result int
	rows.Scan(&result)
	if result != 123 {
		t.Errorf("Expected %d for fetched result, but %d:", 123, result)
	}
}

func TestUpsert(t *testing.T) {
	_, n, _ := Version()
	if !(n >= 3024000) {
		t.Skip("UPSERT requires sqlite3 => 3.24.0")
	}
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec("drop table foo")
	_, err = db.Exec("create table foo (name string primary key, counter integer)")
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	for i := 0; i < 10; i++ {
		res, err := db.Exec("insert into foo(name, counter) values('key', 1) on conflict (name) do update set counter=counter+1")
		if err != nil {
			t.Fatal("Failed to upsert record:", err)
		}
		affected, _ := res.RowsAffected()
		if affected != 1 {
			t.Fatalf("Expected %d for affected rows, but %d:", 1, affected)
		}
	}
	rows, err := db.Query("select name, counter from foo")
	if err != nil {
		t.Fatal("Failed to select records:", err)
	}
	defer rows.Close()

	rows.Next()

	var resultName string
	var resultCounter int
	rows.Scan(&resultName, &resultCounter)
	if resultName != "key" {
		t.Errorf("Expected %s for fetched result, but %s:", "key", resultName)
	}
	if resultCounter != 10 {
		t.Errorf("Expected %d for fetched result, but %d:", 10, resultCounter)
	}

}

func TestUpdate(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec("drop table foo")
	_, err = db.Exec("create table foo (id integer)")
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	res, err := db.Exec("insert into foo(id) values(123)")
	if err != nil {
		t.Fatal("Failed to insert record:", err)
	}
	expected, err := res.LastInsertId()
	if err != nil {
		t.Fatal("Failed to get LastInsertId:", err)
	}
	affected, _ := res.RowsAffected()
	if err != nil {
		t.Fatal("Failed to get RowsAffected:", err)
	}
	if affected != 1 {
		t.Fatalf("Expected %d for affected rows, but %d:", 1, affected)
	}

	res, err = db.Exec("update foo set id = 234")
	if err != nil {
		t.Fatal("Failed to update record:", err)
	}
	lastID, err := res.LastInsertId()
	if err != nil {
		t.Fatal("Failed to get LastInsertId:", err)
	}
	if expected != lastID {
		t.Errorf("Expected %q for last Id, but %q:", expected, lastID)
	}
	affected, _ = res.RowsAffected()
	if err != nil {
		t.Fatal("Failed to get RowsAffected:", err)
	}
	if affected != 1 {
		t.Fatalf("Expected %d for affected rows, but %d:", 1, affected)
	}

	rows, err := db.Query("select id from foo")
	if err != nil {
		t.Fatal("Failed to select records:", err)
	}
	defer rows.Close()

	rows.Next()

	var result int
	rows.Scan(&result)
	if result != 234 {
		t.Errorf("Expected %d for fetched result, but %d:", 234, result)
	}
}

func TestDelete(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec("drop table foo")
	_, err = db.Exec("create table foo (id integer)")
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	res, err := db.Exec("insert into foo(id) values(123)")
	if err != nil {
		t.Fatal("Failed to insert record:", err)
	}
	expected, err := res.LastInsertId()
	if err != nil {
		t.Fatal("Failed to get LastInsertId:", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		t.Fatal("Failed to get RowsAffected:", err)
	}
	if affected != 1 {
		t.Errorf("Expected %d for cout of affected rows, but %q:", 1, affected)
	}

	res, err = db.Exec("delete from foo where id = 123")
	if err != nil {
		t.Fatal("Failed to delete record:", err)
	}
	lastID, err := res.LastInsertId()
	if err != nil {
		t.Fatal("Failed to get LastInsertId:", err)
	}
	if expected != lastID {
		t.Errorf("Expected %q for last Id, but %q:", expected, lastID)
	}
	affected, err = res.RowsAffected()
	if err != nil {
		t.Fatal("Failed to get RowsAffected:", err)
	}
	if affected != 1 {
		t.Errorf("Expected %d for cout of affected rows, but %q:", 1, affected)
	}

	rows, err := db.Query("select id from foo")
	if err != nil {
		t.Fatal("Failed to select records:", err)
	}
	defer rows.Close()

	if rows.Next() {
		t.Error("Fetched row but expected not rows")
	}
}

func TestBooleanRoundtrip(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec("DROP TABLE foo")
	_, err = db.Exec("CREATE TABLE foo(id INTEGER, value BOOL)")
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	_, err = db.Exec("INSERT INTO foo(id, value) VALUES(1, ?)", true)
	if err != nil {
		t.Fatal("Failed to insert true value:", err)
	}

	_, err = db.Exec("INSERT INTO foo(id, value) VALUES(2, ?)", false)
	if err != nil {
		t.Fatal("Failed to insert false value:", err)
	}

	rows, err := db.Query("SELECT id, value FROM foo")
	if err != nil {
		t.Fatal("Unable to query foo table:", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var value bool

		if err := rows.Scan(&id, &value); err != nil {
			t.Error("Unable to scan results:", err)
			continue
		}

		if id == 1 && !value {
			t.Error("Value for id 1 should be true, not false")

		} else if id == 2 && value {
			t.Error("Value for id 2 should be false, not true")
		}
	}
}

func timezone(t time.Time) string { return t.Format("-07:00") }

func TestTimestamp(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec("DROP TABLE foo")
	_, err = db.Exec("CREATE TABLE foo(id INTEGER, ts timeSTAMP, dt DATETIME)")
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	timestamp1 := time.Date(2012, time.April, 6, 22, 50, 0, 0, time.UTC)
	timestamp2 := time.Date(2006, time.January, 2, 15, 4, 5, 123456789, time.UTC)
	timestamp3 := time.Date(2012, time.November, 4, 0, 0, 0, 0, time.UTC)
	tzTest := time.FixedZone("TEST", -9*3600-13*60)
	tests := []struct {
		value    interface{}
		expected time.Time
	}{
		{"nonsense", time.Time{}},
		{"0000-00-00 00:00:00", time.Time{}},
		{time.Time{}.Unix(), time.Time{}},
		{timestamp1, timestamp1},
		{timestamp2.Unix(), timestamp2.Truncate(time.Second)},
		{timestamp2.UnixNano() / int64(time.Millisecond), timestamp2.Truncate(time.Millisecond)},
		{timestamp1.In(tzTest), timestamp1.In(tzTest)},
		{timestamp1.Format("2006-01-02 15:04:05.000"), timestamp1},
		{timestamp1.Format("2006-01-02T15:04:05.000"), timestamp1},
		{timestamp1.Format("2006-01-02 15:04:05"), timestamp1},
		{timestamp1.Format("2006-01-02T15:04:05"), timestamp1},
		{timestamp2, timestamp2},
		{"2006-01-02 15:04:05.123456789", timestamp2},
		{"2006-01-02T15:04:05.123456789", timestamp2},
		{"2006-01-02T05:51:05.123456789-09:13", timestamp2.In(tzTest)},
		{"2012-11-04", timestamp3},
		{"2012-11-04 00:00", timestamp3},
		{"2012-11-04 00:00:00", timestamp3},
		{"2012-11-04 00:00:00.000", timestamp3},
		{"2012-11-04T00:00", timestamp3},
		{"2012-11-04T00:00:00", timestamp3},
		{"2012-11-04T00:00:00.000", timestamp3},
		{"2006-01-02T15:04:05.123456789Z", timestamp2},
		{"2012-11-04Z", timestamp3},
		{"2012-11-04 00:00Z", timestamp3},
		{"2012-11-04 00:00:00Z", timestamp3},
		{"2012-11-04 00:00:00.000Z", timestamp3},
		{"2012-11-04T00:00Z", timestamp3},
		{"2012-11-04T00:00:00Z", timestamp3},
		{"2012-11-04T00:00:00.000Z", timestamp3},
	}
	for i := range tests {
		_, err = db.Exec("INSERT INTO foo(id, ts, dt) VALUES(?, ?, ?)", i, tests[i].value, tests[i].value)
		if err != nil {
			t.Fatal("Failed to insert timestamp:", err)
		}
	}

	rows, err := db.Query("SELECT id, ts, dt FROM foo ORDER BY id ASC")
	if err != nil {
		t.Fatal("Unable to query foo table:", err)
	}
	defer rows.Close()

	seen := 0
	for rows.Next() {
		var id int
		var ts, dt time.Time

		if err := rows.Scan(&id, &ts, &dt); err != nil {
			t.Error("Unable to scan results:", err)
			continue
		}
		if id < 0 || id >= len(tests) {
			t.Error("Bad row id: ", id)
			continue
		}
		seen++
		if !tests[id].expected.Equal(ts) {
			t.Errorf("Timestamp value for id %v (%v) should be %v, not %v", id, tests[id].value, tests[id].expected, dt)
		}
		if !tests[id].expected.Equal(dt) {
			t.Errorf("Datetime value for id %v (%v) should be %v, not %v", id, tests[id].value, tests[id].expected, dt)
		}
		if timezone(tests[id].expected) != timezone(ts) {
			t.Errorf("Timezone for id %v (%v) should be %v, not %v", id, tests[id].value,
				timezone(tests[id].expected), timezone(ts))
		}
		if timezone(tests[id].expected) != timezone(dt) {
			t.Errorf("Timezone for id %v (%v) should be %v, not %v", id, tests[id].value,
				timezone(tests[id].expected), timezone(dt))
		}
	}

	if seen != len(tests) {
		t.Errorf("Expected to see %d rows", len(tests))
	}
}

func TestBoolean(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}

	defer db.Close()

	_, err = db.Exec("CREATE TABLE foo(id INTEGER, fbool BOOLEAN)")
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	bool1 := true
	_, err = db.Exec("INSERT INTO foo(id, fbool) VALUES(1, ?)", bool1)
	if err != nil {
		t.Fatal("Failed to insert boolean:", err)
	}

	bool2 := false
	_, err = db.Exec("INSERT INTO foo(id, fbool) VALUES(2, ?)", bool2)
	if err != nil {
		t.Fatal("Failed to insert boolean:", err)
	}

	bool3 := "nonsense"
	_, err = db.Exec("INSERT INTO foo(id, fbool) VALUES(3, ?)", bool3)
	if err != nil {
		t.Fatal("Failed to insert nonsense:", err)
	}

	rows, err := db.Query("SELECT id, fbool FROM foo where fbool = ?", bool1)
	if err != nil {
		t.Fatal("Unable to query foo table:", err)
	}
	counter := 0

	var id int
	var fbool bool

	for rows.Next() {
		if err := rows.Scan(&id, &fbool); err != nil {
			t.Fatal("Unable to scan results:", err)
		}
		counter++
	}

	if counter != 1 {
		t.Fatalf("Expected 1 row but %v", counter)
	}

	if id != 1 && !fbool {
		t.Fatalf("Value for id 1 should be %v, not %v", bool1, fbool)
	}

	rows, err = db.Query("SELECT id, fbool FROM foo where fbool = ?", bool2)
	if err != nil {
		t.Fatal("Unable to query foo table:", err)
	}

	counter = 0

	for rows.Next() {
		if err := rows.Scan(&id, &fbool); err != nil {
			t.Fatal("Unable to scan results:", err)
		}
		counter++
	}

	if counter != 1 {
		t.Fatalf("Expected 1 row but %v", counter)
	}

	if id != 2 && fbool {
		t.Fatalf("Value for id 2 should be %v, not %v", bool2, fbool)
	}

	// make sure "nonsense" triggered an error
	rows, err = db.Query("SELECT id, fbool FROM foo where id=?;", 3)
	if err != nil {
		t.Fatal("Unable to query foo table:", err)
	}

	rows.Next()
	err = rows.Scan(&id, &fbool)
	if err == nil {
		t.Error("Expected error from \"nonsense\" bool")
	}
}

func TestFloat32(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE foo(id INTEGER)")
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	_, err = db.Exec("INSERT INTO foo(id) VALUES(null)")
	if err != nil {
		t.Fatal("Failed to insert null:", err)
	}

	rows, err := db.Query("SELECT id FROM foo")
	if err != nil {
		t.Fatal("Unable to query foo table:", err)
	}

	if !rows.Next() {
		t.Fatal("Unable to query results:", err)
	}

	var id interface{}
	if err := rows.Scan(&id); err != nil {
		t.Fatal("Unable to scan results:", err)
	}
	if id != nil {
		t.Error("Expected nil but not")
	}
}

func TestNull(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT 3.141592")
	if err != nil {
		t.Fatal("Unable to query foo table:", err)
	}

	if !rows.Next() {
		t.Fatal("Unable to query results:", err)
	}

	var v interface{}
	if err := rows.Scan(&v); err != nil {
		t.Fatal("Unable to scan results:", err)
	}
	f, ok := v.(float64)
	if !ok {
		t.Error("Expected float but not")
	}
	if f != 3.141592 {
		t.Error("Expected 3.141592 but not")
	}
}

func TestTransaction(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE foo(id INTEGER)")
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatal("Failed to begin transaction:", err)
	}

	_, err = tx.Exec("INSERT INTO foo(id) VALUES(1)")
	if err != nil {
		t.Fatal("Failed to insert null:", err)
	}

	rows, err := tx.Query("SELECT id from foo")
	if err != nil {
		t.Fatal("Unable to query foo table:", err)
	}

	err = tx.Rollback()
	if err != nil {
		t.Fatal("Failed to rollback transaction:", err)
	}

	if rows.Next() {
		t.Fatal("Unable to query results:", err)
	}

	tx, err = db.Begin()
	if err != nil {
		t.Fatal("Failed to begin transaction:", err)
	}

	_, err = tx.Exec("INSERT INTO foo(id) VALUES(1)")
	if err != nil {
		t.Fatal("Failed to insert null:", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatal("Failed to commit transaction:", err)
	}

	rows, err = tx.Query("SELECT id from foo")
	if err == nil {
		t.Fatal("Expected failure to query")
	}
}

func TestWAL(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	if _, err = db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		t.Fatal("Failed to Exec PRAGMA journal_mode:", err)
	}
	if _, err = db.Exec("PRAGMA locking_mode=EXCLUSIVE;"); err != nil {
		t.Fatal("Failed to Exec PRAGMA locking_mode:", err)
	}
	if _, err = db.Exec("CREATE TABLE test (id SERIAL, user TEXT NOT NULL, name TEXT NOT NULL);"); err != nil {
		t.Fatal("Failed to Exec CREATE TABLE:", err)
	}
	if _, err = db.Exec("INSERT INTO test (user, name) VALUES ('user','name');"); err != nil {
		t.Fatal("Failed to Exec INSERT:", err)
	}

	trans, err := db.Begin()
	if err != nil {
		t.Fatal("Failed to Begin:", err)
	}
	s, err := trans.Prepare("INSERT INTO test (user, name) VALUES (?, ?);")
	if err != nil {
		t.Fatal("Failed to Prepare:", err)
	}

	var count int
	if err = trans.QueryRow("SELECT count(user) FROM test;").Scan(&count); err != nil {
		t.Fatal("Failed to QueryRow:", err)
	}
	if _, err = s.Exec("bbbb", "aaaa"); err != nil {
		t.Fatal("Failed to Exec prepared statement:", err)
	}
	if err = s.Close(); err != nil {
		t.Fatal("Failed to Close prepared statement:", err)
	}
	if err = trans.Commit(); err != nil {
		t.Fatal("Failed to Commit:", err)
	}
}

func TestTimezoneConversion(t *testing.T) {
	zones := []string{"UTC", "US/Central", "US/Pacific", "Local"}
	for _, tz := range zones {
		tempFilename := TempFilename(t)
		defer os.Remove(tempFilename)
		db, err := sql.Open("sqlite3", tempFilename+"?_loc="+url.QueryEscape(tz))
		if err != nil {
			t.Fatal("Failed to open database:", err)
		}
		defer db.Close()

		_, err = db.Exec("DROP TABLE foo")
		_, err = db.Exec("CREATE TABLE foo(id INTEGER, ts TIMESTAMP, dt DATETIME)")
		if err != nil {
			t.Fatal("Failed to create table:", err)
		}

		loc, err := time.LoadLocation(tz)
		if err != nil {
			t.Fatal("Failed to load location:", err)
		}

		timestamp1 := time.Date(2012, time.April, 6, 22, 50, 0, 0, time.UTC)
		timestamp2 := time.Date(2006, time.January, 2, 15, 4, 5, 123456789, time.UTC)
		timestamp3 := time.Date(2012, time.November, 4, 0, 0, 0, 0, time.UTC)
		tests := []struct {
			value    interface{}
			expected time.Time
		}{
			{"nonsense", time.Time{}.In(loc)},
			{"0000-00-00 00:00:00", time.Time{}.In(loc)},
			{timestamp1, timestamp1.In(loc)},
			{timestamp1.Unix(), timestamp1.In(loc)},
			{timestamp1.In(time.FixedZone("TEST", -7*3600)), timestamp1.In(loc)},
			{timestamp1.Format("2006-01-02 15:04:05.000"), timestamp1.In(loc)},
			{timestamp1.Format("2006-01-02T15:04:05.000"), timestamp1.In(loc)},
			{timestamp1.Format("2006-01-02 15:04:05"), timestamp1.In(loc)},
			{timestamp1.Format("2006-01-02T15:04:05"), timestamp1.In(loc)},
			{timestamp2, timestamp2.In(loc)},
			{"2006-01-02 15:04:05.123456789", timestamp2.In(loc)},
			{"2006-01-02T15:04:05.123456789", timestamp2.In(loc)},
			{"2012-11-04", timestamp3.In(loc)},
			{"2012-11-04 00:00", timestamp3.In(loc)},
			{"2012-11-04 00:00:00", timestamp3.In(loc)},
			{"2012-11-04 00:00:00.000", timestamp3.In(loc)},
			{"2012-11-04T00:00", timestamp3.In(loc)},
			{"2012-11-04T00:00:00", timestamp3.In(loc)},
			{"2012-11-04T00:00:00.000", timestamp3.In(loc)},
		}
		for i := range tests {
			_, err = db.Exec("INSERT INTO foo(id, ts, dt) VALUES(?, ?, ?)", i, tests[i].value, tests[i].value)
			if err != nil {
				t.Fatal("Failed to insert timestamp:", err)
			}
		}

		rows, err := db.Query("SELECT id, ts, dt FROM foo ORDER BY id ASC")
		if err != nil {
			t.Fatal("Unable to query foo table:", err)
		}
		defer rows.Close()

		seen := 0
		for rows.Next() {
			var id int
			var ts, dt time.Time

			if err := rows.Scan(&id, &ts, &dt); err != nil {
				t.Error("Unable to scan results:", err)
				continue
			}
			if id < 0 || id >= len(tests) {
				t.Error("Bad row id: ", id)
				continue
			}
			seen++
			if !tests[id].expected.Equal(ts) {
				t.Errorf("Timestamp value for id %v (%v) should be %v, not %v", id, tests[id].value, tests[id].expected, ts)
			}
			if !tests[id].expected.Equal(dt) {
				t.Errorf("Datetime value for id %v (%v) should be %v, not %v", id, tests[id].value, tests[id].expected, dt)
			}
			if tests[id].expected.Location().String() != ts.Location().String() {
				t.Errorf("Location for id %v (%v) should be %v, not %v", id, tests[id].value, tests[id].expected.Location().String(), ts.Location().String())
			}
			if tests[id].expected.Location().String() != dt.Location().String() {
				t.Errorf("Location for id %v (%v) should be %v, not %v", id, tests[id].value, tests[id].expected.Location().String(), dt.Location().String())
			}
		}

		if seen != len(tests) {
			t.Errorf("Expected to see %d rows", len(tests))
		}
	}
}

// TODO: Execer & Queryer currently disabled
// https://github.com/mattn/go-sqlite3/issues/82
func TestExecer(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec(`
       create table foo (id integer); -- one comment
       insert into foo(id) values(?);
       insert into foo(id) values(?);
       insert into foo(id) values(?); -- another comment
       `, 1, 2, 3)
	if err != nil {
		t.Error("Failed to call db.Exec:", err)
	}
}

func TestQueryer(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec(`
	create table foo (id integer);
	`)
	if err != nil {
		t.Error("Failed to call db.Query:", err)
	}

	rows, err := db.Query(`
	insert into foo(id) values(?);
	insert into foo(id) values(?);
	insert into foo(id) values(?);
	select id from foo order by id;
	`, 3, 2, 1)
	if err != nil {
		t.Error("Failed to call db.Query:", err)
	}
	defer rows.Close()
	n := 1
	if rows != nil {
		for rows.Next() {
			var id int
			err = rows.Scan(&id)
			if err != nil {
				t.Error("Failed to db.Query:", err)
			}
			if id != n {
				t.Error("Failed to db.Query: not matched results")
			}
		}
	}
}

func TestStress(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	db.Exec("CREATE TABLE foo (id int);")
	db.Exec("INSERT INTO foo VALUES(1);")
	db.Exec("INSERT INTO foo VALUES(2);")
	db.Close()

	for i := 0; i < 10000; i++ {
		db, err := sql.Open("sqlite3", tempFilename)
		if err != nil {
			t.Fatal("Failed to open database:", err)
		}

		for j := 0; j < 3; j++ {
			rows, err := db.Query("select * from foo where id=1;")
			if err != nil {
				t.Error("Failed to call db.Query:", err)
			}
			for rows.Next() {
				var i int
				if err := rows.Scan(&i); err != nil {
					t.Errorf("Scan failed: %v\n", err)
				}
			}
			if err := rows.Err(); err != nil {
				t.Errorf("Post-scan failed: %v\n", err)
			}
			rows.Close()
		}
		db.Close()
	}
}

func TestDateTimeLocal(t *testing.T) {
	zone := "Asia/Tokyo"
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename+"?_loc="+zone)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	db.Exec("CREATE TABLE foo (dt datetime);")
	db.Exec("INSERT INTO foo VALUES('2015-03-05 15:16:17');")

	row := db.QueryRow("select * from foo")
	var d time.Time
	err = row.Scan(&d)
	if err != nil {
		t.Fatal("Failed to scan datetime:", err)
	}
	if d.Hour() == 15 || !strings.Contains(d.String(), "JST") {
		t.Fatal("Result should have timezone", d)
	}
	db.Close()

	db, err = sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}

	row = db.QueryRow("select * from foo")
	err = row.Scan(&d)
	if err != nil {
		t.Fatal("Failed to scan datetime:", err)
	}
	if d.UTC().Hour() != 15 || !strings.Contains(d.String(), "UTC") {
		t.Fatalf("Result should not have timezone %v %v", zone, d.String())
	}

	_, err = db.Exec("DELETE FROM foo")
	if err != nil {
		t.Fatal("Failed to delete table:", err)
	}
	dt, err := time.Parse("2006/1/2 15/4/5 -0700 MST", "2015/3/5 15/16/17 +0900 JST")
	if err != nil {
		t.Fatal("Failed to parse datetime:", err)
	}
	db.Exec("INSERT INTO foo VALUES(?);", dt)

	db.Close()
	db, err = sql.Open("sqlite3", tempFilename+"?_loc="+zone)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}

	row = db.QueryRow("select * from foo")
	err = row.Scan(&d)
	if err != nil {
		t.Fatal("Failed to scan datetime:", err)
	}
	if d.Hour() != 15 || !strings.Contains(d.String(), "JST") {
		t.Fatalf("Result should have timezone %v %v", zone, d.String())
	}
}

func TestVersion(t *testing.T) {
	s, n, id := Version()
	if s == "" || n == 0 || id == "" {
		t.Errorf("Version failed %q, %d, %q\n", s, n, id)
	}
}

func TestStringContainingZero(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec(`
	create table foo (id integer, name, extra text);
	`)
	if err != nil {
		t.Error("Failed to call db.Query:", err)
	}

	const text = "foo\x00bar"

	_, err = db.Exec(`insert into foo(id, name, extra) values($1, $2, $2)`, 1, text)
	if err != nil {
		t.Error("Failed to call db.Exec:", err)
	}

	row := db.QueryRow(`select id, extra from foo where id = $1 and extra = $2`, 1, text)
	if row == nil {
		t.Error("Failed to call db.QueryRow")
	}

	var id int
	var extra string
	err = row.Scan(&id, &extra)
	if err != nil {
		t.Error("Failed to db.Scan:", err)
	}
	if id != 1 || extra != text {
		t.Error("Failed to db.QueryRow: not matched results")
	}
}

const CurrentTimeStamp = "2006-01-02 15:04:05"

type TimeStamp struct{ *time.Time }

func (t TimeStamp) Scan(value interface{}) error {
	var err error
	switch v := value.(type) {
	case string:
		*t.Time, err = time.Parse(CurrentTimeStamp, v)
	case []byte:
		*t.Time, err = time.Parse(CurrentTimeStamp, string(v))
	default:
		err = errors.New("invalid type for current_timestamp")
	}
	return err
}

func (t TimeStamp) Value() (driver.Value, error) {
	return t.Time.Format(CurrentTimeStamp), nil
}

func TestDateTimeNow(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	db, err := sql.Open("sqlite3", tempFilename)
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	var d time.Time
	err = db.QueryRow("SELECT datetime('now')").Scan(TimeStamp{&d})
	if err != nil {
		t.Fatal("Failed to scan datetime:", err)
	}
}

func TestFunctionRegistration(t *testing.T) {
	addi8_16_32 := func(a int8, b int16) int32 { return int32(a) + int32(b) }
	addi64 := func(a, b int64) int64 { return a + b }
	addu8_16_32 := func(a uint8, b uint16) uint32 { return uint32(a) + uint32(b) }
	addu64 := func(a, b uint64) uint64 { return a + b }
	addiu := func(a int, b uint) int64 { return int64(a) + int64(b) }
	addf32_64 := func(a float32, b float64) float64 { return float64(a) + b }
	not := func(a bool) bool { return !a }
	regex := func(re, s string) (bool, error) {
		return regexp.MatchString(re, s)
	}
	generic := func(a interface{}) int64 {
		switch a.(type) {
		case int64:
			return 1
		case float64:
			return 2
		case []byte:
			return 3
		case string:
			return 4
		default:
			panic("unreachable")
		}
	}
	variadic := func(a, b int64, c ...int64) int64 {
		ret := a + b
		for _, d := range c {
			ret += d
		}
		return ret
	}
	variadicGeneric := func(a ...interface{}) int64 {
		return int64(len(a))
	}

	sql.Register("sqlite3_FunctionRegistration", &SQLiteDriver{
		ConnectHook: func(conn *SQLiteConn) error {
			if err := conn.RegisterFunc("addi8_16_32", addi8_16_32, true); err != nil {
				return err
			}
			if err := conn.RegisterFunc("addi64", addi64, true); err != nil {
				return err
			}
			if err := conn.RegisterFunc("addu8_16_32", addu8_16_32, true); err != nil {
				return err
			}
			if err := conn.RegisterFunc("addu64", addu64, true); err != nil {
				return err
			}
			if err := conn.RegisterFunc("addiu", addiu, true); err != nil {
				return err
			}
			if err := conn.RegisterFunc("addf32_64", addf32_64, true); err != nil {
				return err
			}
			if err := conn.RegisterFunc("not", not, true); err != nil {
				return err
			}
			if err := conn.RegisterFunc("regex", regex, true); err != nil {
				return err
			}
			if err := conn.RegisterFunc("generic", generic, true); err != nil {
				return err
			}
			if err := conn.RegisterFunc("variadic", variadic, true); err != nil {
				return err
			}
			if err := conn.RegisterFunc("variadicGeneric", variadicGeneric, true); err != nil {
				return err
			}
			return nil
		},
	})
	db, err := sql.Open("sqlite3_FunctionRegistration", ":memory:")
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	ops := []struct {
		query    string
		expected interface{}
	}{
		{"SELECT addi8_16_32(1,2)", int32(3)},
		{"SELECT addi64(1,2)", int64(3)},
		{"SELECT addu8_16_32(1,2)", uint32(3)},
		{"SELECT addu64(1,2)", uint64(3)},
		{"SELECT addiu(1,2)", int64(3)},
		{"SELECT addf32_64(1.5,1.5)", float64(3)},
		{"SELECT not(1)", false},
		{"SELECT not(0)", true},
		{`SELECT regex("^foo.*", "foobar")`, true},
		{`SELECT regex("^foo.*", "barfoobar")`, false},
		{"SELECT generic(1)", int64(1)},
		{"SELECT generic(1.1)", int64(2)},
		{`SELECT generic(NULL)`, int64(3)},
		{`SELECT generic("foo")`, int64(4)},
		{"SELECT variadic(1,2)", int64(3)},
		{"SELECT variadic(1,2,3,4)", int64(10)},
		{"SELECT variadic(1,1,1,1,1,1,1,1,1,1)", int64(10)},
		{`SELECT variadicGeneric(1,"foo",2.3, NULL)`, int64(4)},
	}

	for _, op := range ops {
		ret := reflect.New(reflect.TypeOf(op.expected))
		err = db.QueryRow(op.query).Scan(ret.Interface())
		if err != nil {
			t.Errorf("Query %q failed: %s", op.query, err)
		} else if !reflect.DeepEqual(ret.Elem().Interface(), op.expected) {
			t.Errorf("Query %q returned wrong value: got %v (%T), want %v (%T)", op.query, ret.Elem().Interface(), ret.Elem().Interface(), op.expected, op.expected)
		}
	}
}

type sumAggregator int64

func (s *sumAggregator) Step(x int64) {
	*s += sumAggregator(x)
}

func (s *sumAggregator) Done() int64 {
	return int64(*s)
}

func TestAggregatorRegistration(t *testing.T) {
	customSum := func() *sumAggregator {
		var ret sumAggregator
		return &ret
	}

	sql.Register("sqlite3_AggregatorRegistration", &SQLiteDriver{
		ConnectHook: func(conn *SQLiteConn) error {
			if err := conn.RegisterAggregator("customSum", customSum, true); err != nil {
				return err
			}
			return nil
		},
	})
	db, err := sql.Open("sqlite3_AggregatorRegistration", ":memory:")
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	_, err = db.Exec("create table foo (department integer, profits integer)")
	if err != nil {
		// trace feature is not implemented
		t.Skip("Failed to create table:", err)
	}

	_, err = db.Exec("insert into foo values (1, 10), (1, 20), (2, 42)")
	if err != nil {
		t.Fatal("Failed to insert records:", err)
	}

	tests := []struct {
		dept, sum int64
	}{
		{1, 30},
		{2, 42},
	}

	for _, test := range tests {
		var ret int64
		err = db.QueryRow("select customSum(profits) from foo where department = $1 group by department", test.dept).Scan(&ret)
		if err != nil {
			t.Fatal("Query failed:", err)
		}
		if ret != test.sum {
			t.Fatalf("Custom sum returned wrong value, got %d, want %d", ret, test.sum)
		}
	}
}

func rot13(r rune) rune {
	switch {
	case r >= 'A' && r <= 'Z':
		return 'A' + (r-'A'+13)%26
	case r >= 'a' && r <= 'z':
		return 'a' + (r-'a'+13)%26
	}
	return r
}

func TestCollationRegistration(t *testing.T) {
	collateRot13 := func(a, b string) int {
		ra, rb := strings.Map(rot13, a), strings.Map(rot13, b)
		return strings.Compare(ra, rb)
	}
	collateRot13Reverse := func(a, b string) int {
		return collateRot13(b, a)
	}

	sql.Register("sqlite3_CollationRegistration", &SQLiteDriver{
		ConnectHook: func(conn *SQLiteConn) error {
			if err := conn.RegisterCollation("rot13", collateRot13); err != nil {
				return err
			}
			if err := conn.RegisterCollation("rot13reverse", collateRot13Reverse); err != nil {
				return err
			}
			return nil
		},
	})

	db, err := sql.Open("sqlite3_CollationRegistration", ":memory:")
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	populate := []string{
		`CREATE TABLE test (s TEXT)`,
		`INSERT INTO test VALUES ("aaaa")`,
		`INSERT INTO test VALUES ("ffff")`,
		`INSERT INTO test VALUES ("qqqq")`,
		`INSERT INTO test VALUES ("tttt")`,
		`INSERT INTO test VALUES ("zzzz")`,
	}
	for _, stmt := range populate {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal("Failed to populate test DB:", err)
		}
	}

	ops := []struct {
		query string
		want  []string
	}{
		{
			"SELECT * FROM test ORDER BY s COLLATE rot13 ASC",
			[]string{
				"qqqq",
				"tttt",
				"zzzz",
				"aaaa",
				"ffff",
			},
		},
		{
			"SELECT * FROM test ORDER BY s COLLATE rot13 DESC",
			[]string{
				"ffff",
				"aaaa",
				"zzzz",
				"tttt",
				"qqqq",
			},
		},
		{
			"SELECT * FROM test ORDER BY s COLLATE rot13reverse ASC",
			[]string{
				"ffff",
				"aaaa",
				"zzzz",
				"tttt",
				"qqqq",
			},
		},
		{
			"SELECT * FROM test ORDER BY s COLLATE rot13reverse DESC",
			[]string{
				"qqqq",
				"tttt",
				"zzzz",
				"aaaa",
				"ffff",
			},
		},
	}

	for _, op := range ops {
		rows, err := db.Query(op.query)
		if err != nil {
			t.Fatalf("Query %q failed: %s", op.query, err)
		}
		got := []string{}
		defer rows.Close()
		for rows.Next() {
			var s string
			if err = rows.Scan(&s); err != nil {
				t.Fatalf("Reading row for %q: %s", op.query, err)
			}
			got = append(got, s)
		}
		if err = rows.Err(); err != nil {
			t.Fatalf("Reading rows for %q: %s", op.query, err)
		}

		if !reflect.DeepEqual(got, op.want) {
			t.Fatalf("Unexpected output from %q\ngot:\n%s\n\nwant:\n%s", op.query, strings.Join(got, "\n"), strings.Join(op.want, "\n"))
		}
	}
}

func TestDeclTypes(t *testing.T) {

	d := SQLiteDriver{}

	conn, err := d.Open(":memory:")
	if err != nil {
		t.Fatal("Failed to begin transaction:", err)
	}
	defer conn.Close()

	sqlite3conn := conn.(*SQLiteConn)

	_, err = sqlite3conn.Exec("create table foo (id integer not null primary key, name text)", nil)
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	_, err = sqlite3conn.Exec("insert into foo(name) values(\"bar\")", nil)
	if err != nil {
		t.Fatal("Failed to insert:", err)
	}

	rs, err := sqlite3conn.Query("select * from foo", nil)
	if err != nil {
		t.Fatal("Failed to select:", err)
	}
	defer rs.Close()

	declTypes := rs.(*SQLiteRows).DeclTypes()

	if !reflect.DeepEqual(declTypes, []string{"integer", "text"}) {
		t.Fatal("Unexpected declTypes:", declTypes)
	}
}

func TestPinger(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	err = db.Ping()
	if err == nil {
		t.Fatal("Should be closed")
	}
}

func TestUpdateAndTransactionHooks(t *testing.T) {
	var events []string
	var commitHookReturn = 0

	sql.Register("sqlite3_UpdateHook", &SQLiteDriver{
		ConnectHook: func(conn *SQLiteConn) error {
			conn.RegisterCommitHook(func() int {
				events = append(events, "commit")
				return commitHookReturn
			})
			conn.RegisterRollbackHook(func() {
				events = append(events, "rollback")
			})
			conn.RegisterUpdateHook(func(op int, db string, table string, rowid int64) {
				events = append(events, fmt.Sprintf("update(op=%v db=%v table=%v rowid=%v)", op, db, table, rowid))
			})
			return nil
		},
	})
	db, err := sql.Open("sqlite3_UpdateHook", ":memory:")
	if err != nil {
		t.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	statements := []string{
		"create table foo (id integer primary key)",
		"insert into foo values (9)",
		"update foo set id = 99 where id = 9",
		"delete from foo where id = 99",
	}
	for _, statement := range statements {
		_, err = db.Exec(statement)
		if err != nil {
			t.Fatalf("Unable to prepare test data [%v]: %v", statement, err)
		}
	}

	commitHookReturn = 1
	_, err = db.Exec("insert into foo values (5)")
	if err == nil {
		t.Error("Commit hook failed to rollback transaction")
	}

	var expected = []string{
		"commit",
		fmt.Sprintf("update(op=%v db=main table=foo rowid=9)", SQLITE_INSERT),
		"commit",
		fmt.Sprintf("update(op=%v db=main table=foo rowid=99)", SQLITE_UPDATE),
		"commit",
		fmt.Sprintf("update(op=%v db=main table=foo rowid=99)", SQLITE_DELETE),
		"commit",
		fmt.Sprintf("update(op=%v db=main table=foo rowid=5)", SQLITE_INSERT),
		"commit",
		"rollback",
	}
	if !reflect.DeepEqual(events, expected) {
		t.Errorf("Expected notifications %v but got %v", expected, events)
	}
}

func TestNilAndEmptyBytes(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	actualNil := []byte("use this to use an actual nil not a reference to nil")
	emptyBytes := []byte{}
	for tsti, tst := range []struct {
		name          string
		columnType    string
		insertBytes   []byte
		expectedBytes []byte
	}{
		{"actual nil blob", "blob", actualNil, nil},
		{"referenced nil blob", "blob", nil, nil},
		{"empty blob", "blob", emptyBytes, emptyBytes},
		{"actual nil text", "text", actualNil, nil},
		{"referenced nil text", "text", nil, nil},
		{"empty text", "text", emptyBytes, emptyBytes},
	} {
		if _, err = db.Exec(fmt.Sprintf("create table tbl%d (txt %s)", tsti, tst.columnType)); err != nil {
			t.Fatal(tst.name, err)
		}
		if bytes.Equal(tst.insertBytes, actualNil) {
			if _, err = db.Exec(fmt.Sprintf("insert into tbl%d (txt) values (?)", tsti), nil); err != nil {
				t.Fatal(tst.name, err)
			}
		} else {
			if _, err = db.Exec(fmt.Sprintf("insert into tbl%d (txt) values (?)", tsti), &tst.insertBytes); err != nil {
				t.Fatal(tst.name, err)
			}
		}
		rows, err := db.Query(fmt.Sprintf("select txt from tbl%d", tsti))
		if err != nil {
			t.Fatal(tst.name, err)
		}
		if !rows.Next() {
			t.Fatal(tst.name, "no rows")
		}
		var scanBytes []byte
		if err = rows.Scan(&scanBytes); err != nil {
			t.Fatal(tst.name, err)
		}
		if err = rows.Err(); err != nil {
			t.Fatal(tst.name, err)
		}
		if tst.expectedBytes == nil && scanBytes != nil {
			t.Errorf("%s: %#v != %#v", tst.name, scanBytes, tst.expectedBytes)
		} else if !bytes.Equal(scanBytes, tst.expectedBytes) {
			t.Errorf("%s: %#v != %#v", tst.name, scanBytes, tst.expectedBytes)
		}
	}
}

func TestInsertNilByteSlice(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("create table blob_not_null (b blob not null)"); err != nil {
		t.Fatal(err)
	}
	var nilSlice []byte
	if _, err := db.Exec("insert into blob_not_null (b) values (?)", nilSlice); err == nil {
		t.Fatal("didn't expect INSERT to 'not null' column with a nil []byte slice to work")
	}
	zeroLenSlice := []byte{}
	if _, err := db.Exec("insert into blob_not_null (b) values (?)", zeroLenSlice); err != nil {
		t.Fatal("failed to insert zero-length slice")
	}
}

var customFunctionOnce sync.Once

func BenchmarkCustomFunctions(b *testing.B) {
	customFunctionOnce.Do(func() {
		customAdd := func(a, b int64) int64 {
			return a + b
		}

		sql.Register("sqlite3_BenchmarkCustomFunctions", &SQLiteDriver{
			ConnectHook: func(conn *SQLiteConn) error {
				// Impure function to force sqlite to reexecute it each time.
				return conn.RegisterFunc("custom_add", customAdd, false)
			},
		})
	})

	db, err := sql.Open("sqlite3_BenchmarkCustomFunctions", ":memory:")
	if err != nil {
		b.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var i int64
		err = db.QueryRow("SELECT custom_add(1,2)").Scan(&i)
		if err != nil {
			b.Fatal("Failed to run custom add:", err)
		}
	}
}

func TestSuite(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	d, err := sql.Open("sqlite3", tempFilename+"?_busy_timeout=99999")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	db = &TestDB{t, d, SQLITE, sync.Once{}}
	testing.RunTests(func(string, string) (bool, error) { return true, nil }, tests)

	if !testing.Short() {
		for _, b := range benchmarks {
			fmt.Printf("%-20s", b.Name)
			r := testing.Benchmark(b.F)
			fmt.Printf("%10d %10.0f req/s\n", r.N, float64(r.N)/r.T.Seconds())
		}
	}
	db.tearDown()
}

// Dialect is a type of dialect of databases.
type Dialect int

// Dialects for databases.
const (
	SQLITE     Dialect = iota // SQLITE mean SQLite3 dialect
	POSTGRESQL                // POSTGRESQL mean PostgreSQL dialect
	MYSQL                     // MYSQL mean MySQL dialect
)

// DB provide context for the tests
type TestDB struct {
	*testing.T
	*sql.DB
	dialect Dialect
	once    sync.Once
}

var db *TestDB

// the following tables will be created and dropped during the test
var testTables = []string{"foo", "bar", "t", "bench"}

var tests = []testing.InternalTest{
	{Name: "TestResult", F: testResult},
	{Name: "TestBlobs", F: testBlobs},
	{Name: "TestMultiBlobs", F: testMultiBlobs},
	{Name: "TestManyQueryRow", F: testManyQueryRow},
	{Name: "TestTxQuery", F: testTxQuery},
	{Name: "TestPreparedStmt", F: testPreparedStmt},
}

var benchmarks = []testing.InternalBenchmark{
	{Name: "BenchmarkExec", F: benchmarkExec},
	{Name: "BenchmarkQuery", F: benchmarkQuery},
	{Name: "BenchmarkParams", F: benchmarkParams},
	{Name: "BenchmarkStmt", F: benchmarkStmt},
	{Name: "BenchmarkRows", F: benchmarkRows},
	{Name: "BenchmarkStmtRows", F: benchmarkStmtRows},
}

func (db *TestDB) mustExec(sql string, args ...interface{}) sql.Result {
	res, err := db.Exec(sql, args...)
	if err != nil {
		db.Fatalf("Error running %q: %v", sql, err)
	}
	return res
}

func (db *TestDB) tearDown() {
	for _, tbl := range testTables {
		switch db.dialect {
		case SQLITE:
			db.mustExec("drop table if exists " + tbl)
		case MYSQL, POSTGRESQL:
			db.mustExec("drop table if exists " + tbl)
		default:
			db.Fatal("unknown dialect")
		}
	}
}

// q replaces ? parameters if needed
func (db *TestDB) q(sql string) string {
	switch db.dialect {
	case POSTGRESQL: // replace with $1, $2, ..
		qrx := regexp.MustCompile(`\?`)
		n := 0
		return qrx.ReplaceAllStringFunc(sql, func(string) string {
			n++
			return "$" + strconv.Itoa(n)
		})
	}
	return sql
}

func (db *TestDB) blobType(size int) string {
	switch db.dialect {
	case SQLITE:
		return fmt.Sprintf("blob[%d]", size)
	case POSTGRESQL:
		return "bytea"
	case MYSQL:
		return fmt.Sprintf("VARBINARY(%d)", size)
	}
	panic("unknown dialect")
}

func (db *TestDB) serialPK() string {
	switch db.dialect {
	case SQLITE:
		return "integer primary key autoincrement"
	case POSTGRESQL:
		return "serial primary key"
	case MYSQL:
		return "integer primary key auto_increment"
	}
	panic("unknown dialect")
}

func (db *TestDB) now() string {
	switch db.dialect {
	case SQLITE:
		return "datetime('now')"
	case POSTGRESQL:
		return "now()"
	case MYSQL:
		return "now()"
	}
	panic("unknown dialect")
}

func makeBench() {
	if _, err := db.Exec("create table bench (n varchar(32), i integer, d double, s varchar(32), t datetime)"); err != nil {
		panic(err)
	}
	st, err := db.Prepare("insert into bench values (?, ?, ?, ?, ?)")
	if err != nil {
		panic(err)
	}
	defer st.Close()
	for i := 0; i < 100; i++ {
		if _, err = st.Exec(nil, i, float64(i), fmt.Sprintf("%d", i), time.Now()); err != nil {
			panic(err)
		}
	}
}

// testResult is test for result
func testResult(t *testing.T) {
	db.tearDown()
	db.mustExec("create temporary table test (id " + db.serialPK() + ", name varchar(10))")

	for i := 1; i < 3; i++ {
		r := db.mustExec(db.q("insert into test (name) values (?)"), fmt.Sprintf("row %d", i))
		n, err := r.RowsAffected()
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("got %v, want %v", n, 1)
		}
		n, err = r.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if n != int64(i) {
			t.Errorf("got %v, want %v", n, i)
		}
	}
	if _, err := db.Exec("error!"); err == nil {
		t.Fatalf("expected error")
	}
}

// testBlobs is test for blobs
func testBlobs(t *testing.T) {
	db.tearDown()
	var blob = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	db.mustExec("create table foo (id integer primary key, bar " + db.blobType(16) + ")")
	db.mustExec(db.q("insert into foo (id, bar) values(?,?)"), 0, blob)

	want := fmt.Sprintf("%x", blob)

	b := make([]byte, 16)
	err := db.QueryRow(db.q("select bar from foo where id = ?"), 0).Scan(&b)
	got := fmt.Sprintf("%x", b)
	if err != nil {
		t.Errorf("[]byte scan: %v", err)
	} else if got != want {
		t.Errorf("for []byte, got %q; want %q", got, want)
	}

	err = db.QueryRow(db.q("select bar from foo where id = ?"), 0).Scan(&got)
	want = string(blob)
	if err != nil {
		t.Errorf("string scan: %v", err)
	} else if got != want {
		t.Errorf("for string, got %q; want %q", got, want)
	}
}

func testMultiBlobs(t *testing.T) {
	db.tearDown()
	db.mustExec("create table foo (id integer primary key, bar " + db.blobType(16) + ")")
	var blob0 = []byte{0, 1, 2, 3, 4, 5, 6, 7}
	db.mustExec(db.q("insert into foo (id, bar) values(?,?)"), 0, blob0)
	var blob1 = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	db.mustExec(db.q("insert into foo (id, bar) values(?,?)"), 1, blob1)

	r, err := db.Query(db.q("select bar from foo order by id"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !r.Next() {
		if r.Err() != nil {
			t.Fatal(err)
		}
		t.Fatal("expected one rows")
	}

	want0 := fmt.Sprintf("%x", blob0)
	b0 := make([]byte, 8)
	err = r.Scan(&b0)
	if err != nil {
		t.Fatal(err)
	}
	got0 := fmt.Sprintf("%x", b0)

	if !r.Next() {
		if r.Err() != nil {
			t.Fatal(err)
		}
		t.Fatal("expected one rows")
	}

	want1 := fmt.Sprintf("%x", blob1)
	b1 := make([]byte, 16)
	err = r.Scan(&b1)
	if err != nil {
		t.Fatal(err)
	}
	got1 := fmt.Sprintf("%x", b1)
	if got0 != want0 {
		t.Errorf("for []byte, got %q; want %q", got0, want0)
	}
	if got1 != want1 {
		t.Errorf("for []byte, got %q; want %q", got1, want1)
	}
}

// testManyQueryRow is test for many query row
func testManyQueryRow(t *testing.T) {
	if testing.Short() {
		t.Log("skipping in short mode")
		return
	}
	db.tearDown()
	db.mustExec("create table foo (id integer primary key, name varchar(50))")
	db.mustExec(db.q("insert into foo (id, name) values(?,?)"), 1, "bob")
	var name string
	for i := 0; i < 10000; i++ {
		err := db.QueryRow(db.q("select name from foo where id = ?"), 1).Scan(&name)
		if err != nil || name != "bob" {
			t.Fatalf("on query %d: err=%v, name=%q", i, err, name)
		}
	}
}

// testTxQuery is test for transactional query
func testTxQuery(t *testing.T) {
	db.tearDown()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("create table foo (id integer primary key, name varchar(50))")
	if err != nil {
		t.Fatal(err)
	}

	_, err = tx.Exec(db.q("insert into foo (id, name) values(?,?)"), 1, "bob")
	if err != nil {
		t.Fatal(err)
	}

	r, err := tx.Query(db.q("select name from foo where id = ?"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	if !r.Next() {
		if r.Err() != nil {
			t.Fatal(err)
		}
		t.Fatal("expected one rows")
	}

	var name string
	err = r.Scan(&name)
	if err != nil {
		t.Fatal(err)
	}
}

// testPreparedStmt is test for prepared statement
func testPreparedStmt(t *testing.T) {
	db.tearDown()
	db.mustExec("CREATE TABLE t (count INT)")
	sel, err := db.Prepare("SELECT count FROM t ORDER BY count DESC")
	if err != nil {
		t.Fatalf("prepare 1: %v", err)
	}
	ins, err := db.Prepare(db.q("INSERT INTO t (count) VALUES (?)"))
	if err != nil {
		t.Fatalf("prepare 2: %v", err)
	}

	for n := 1; n <= 3; n++ {
		if _, err := ins.Exec(n); err != nil {
			t.Fatalf("insert(%d) = %v", n, err)
		}
	}

	const nRuns = 10
	var wg sync.WaitGroup
	for i := 0; i < nRuns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				count := 0
				if err := sel.QueryRow().Scan(&count); err != nil && err != sql.ErrNoRows {
					t.Errorf("Query: %v", err)
					return
				}
				if _, err := ins.Exec(rand.Intn(100)); err != nil {
					t.Errorf("Insert: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// Benchmarks need to use panic() since b.Error errors are lost when
// running via testing.Benchmark() I would like to run these via go
// test -bench but calling Benchmark() from a benchmark test
// currently hangs go.

// benchmarkExec is benchmark for exec
func benchmarkExec(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := db.Exec("select 1"); err != nil {
			panic(err)
		}
	}
}

// benchmarkQuery is benchmark for query
func benchmarkQuery(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var n sql.NullString
		var i int
		var f float64
		var s string
		//		var t time.Time
		if err := db.QueryRow("select null, 1, 1.1, 'foo'").Scan(&n, &i, &f, &s); err != nil {
			panic(err)
		}
	}
}

// benchmarkParams is benchmark for params
func benchmarkParams(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var n sql.NullString
		var i int
		var f float64
		var s string
		//		var t time.Time
		if err := db.QueryRow("select ?, ?, ?, ?", nil, 1, 1.1, "foo").Scan(&n, &i, &f, &s); err != nil {
			panic(err)
		}
	}
}

// benchmarkStmt is benchmark for statement
func benchmarkStmt(b *testing.B) {
	st, err := db.Prepare("select ?, ?, ?, ?")
	if err != nil {
		panic(err)
	}
	defer st.Close()

	for n := 0; n < b.N; n++ {
		var n sql.NullString
		var i int
		var f float64
		var s string
		//		var t time.Time
		if err := st.QueryRow(nil, 1, 1.1, "foo").Scan(&n, &i, &f, &s); err != nil {
			panic(err)
		}
	}
}

// benchmarkRows is benchmark for rows
func benchmarkRows(b *testing.B) {
	db.once.Do(makeBench)

	for n := 0; n < b.N; n++ {
		var n sql.NullString
		var i int
		var f float64
		var s string
		var t time.Time
		r, err := db.Query("select * from bench")
		if err != nil {
			panic(err)
		}
		for r.Next() {
			if err = r.Scan(&n, &i, &f, &s, &t); err != nil {
				panic(err)
			}
		}
		if err = r.Err(); err != nil {
			panic(err)
		}
	}
}

// benchmarkStmtRows is benchmark for statement rows
func benchmarkStmtRows(b *testing.B) {
	db.once.Do(makeBench)

	st, err := db.Prepare("select * from bench")
	if err != nil {
		panic(err)
	}
	defer st.Close()

	for n := 0; n < b.N; n++ {
		var n sql.NullString
		var i int
		var f float64
		var s string
		var t time.Time
		r, err := st.Query()
		if err != nil {
			panic(err)
		}
		for r.Next() {
			if err = r.Scan(&n, &i, &f, &s, &t); err != nil {
				panic(err)
			}
		}
		if err = r.Err(); err != nil {
			panic(err)
		}
	}
}
