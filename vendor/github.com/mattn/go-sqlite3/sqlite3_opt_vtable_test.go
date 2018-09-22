// Copyright (C) 2014 Yasuhiro Matsumoto <mattn.jp@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// +build sqlite_vtable vtable

package sqlite3

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

type testModule struct {
	t        *testing.T
	intarray []int
}

type testVTab struct {
	intarray []int
}

type testVTabCursor struct {
	vTab  *testVTab
	index int
}

func (m testModule) Create(c *SQLiteConn, args []string) (VTab, error) {
	if len(args) != 6 {
		m.t.Fatal("six arguments expected")
	}
	if args[0] != "test" {
		m.t.Fatal("module name")
	}
	if args[1] != "main" {
		m.t.Fatal("db name")
	}
	if args[2] != "vtab" {
		m.t.Fatal("table name")
	}
	if args[3] != "'1'" {
		m.t.Fatal("first arg")
	}
	if args[4] != "2" {
		m.t.Fatal("second arg")
	}
	if args[5] != "three" {
		m.t.Fatal("third argsecond arg")
	}
	err := c.DeclareVTab("CREATE TABLE x(test TEXT)")
	if err != nil {
		return nil, err
	}
	return &testVTab{m.intarray}, nil
}

func (m testModule) Connect(c *SQLiteConn, args []string) (VTab, error) {
	return m.Create(c, args)
}

func (m testModule) DestroyModule() {}

func (v *testVTab) BestIndex(cst []InfoConstraint, ob []InfoOrderBy) (*IndexResult, error) {
	used := make([]bool, 0, len(cst))
	for range cst {
		used = append(used, false)
	}
	return &IndexResult{
		Used:           used,
		IdxNum:         0,
		IdxStr:         "test-index",
		AlreadyOrdered: true,
		EstimatedCost:  100,
		EstimatedRows:  200,
	}, nil
}

func (v *testVTab) Disconnect() error {
	return nil
}

func (v *testVTab) Destroy() error {
	return nil
}

func (v *testVTab) Open() (VTabCursor, error) {
	return &testVTabCursor{v, 0}, nil
}

func (vc *testVTabCursor) Close() error {
	return nil
}

func (vc *testVTabCursor) Filter(idxNum int, idxStr string, vals []interface{}) error {
	vc.index = 0
	return nil
}

func (vc *testVTabCursor) Next() error {
	vc.index++
	return nil
}

func (vc *testVTabCursor) EOF() bool {
	return vc.index >= len(vc.vTab.intarray)
}

func (vc *testVTabCursor) Column(c *SQLiteContext, col int) error {
	if col != 0 {
		return fmt.Errorf("column index out of bounds: %d", col)
	}
	c.ResultInt(vc.vTab.intarray[vc.index])
	return nil
}

func (vc *testVTabCursor) Rowid() (int64, error) {
	return int64(vc.index), nil
}

func TestCreateModule(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)
	intarray := []int{1, 2, 3}
	sql.Register("sqlite3_TestCreateModule", &SQLiteDriver{
		ConnectHook: func(conn *SQLiteConn) error {
			return conn.CreateModule("test", testModule{t, intarray})
		},
	})
	db, err := sql.Open("sqlite3_TestCreateModule", tempFilename)
	if err != nil {
		t.Fatalf("could not open db: %v", err)
	}
	_, err = db.Exec("CREATE VIRTUAL TABLE vtab USING test('1', 2, three)")
	if err != nil {
		t.Fatalf("could not create vtable: %v", err)
	}

	var i, value int
	rows, err := db.Query("SELECT rowid, * FROM vtab WHERE test = '3'")
	if err != nil {
		t.Fatalf("couldn't select from virtual table: %v", err)
	}
	for rows.Next() {
		rows.Scan(&i, &value)
		if intarray[i] != value {
			t.Fatalf("want %v but %v", intarray[i], value)
		}
	}

	_, err = db.Exec("DROP TABLE vtab")
	if err != nil {
		t.Fatalf("couldn't drop virtual table: %v", err)
	}
}

func TestVUpdate(t *testing.T) {
	tempFilename := TempFilename(t)
	defer os.Remove(tempFilename)

	// create module
	updateMod := &vtabUpdateModule{t, make(map[string]*vtabUpdateTable)}

	// register module
	sql.Register("sqlite3_TestVUpdate", &SQLiteDriver{
		ConnectHook: func(conn *SQLiteConn) error {
			return conn.CreateModule("updatetest", updateMod)
		},
	})

	// connect
	db, err := sql.Open("sqlite3_TestVUpdate", tempFilename)
	if err != nil {
		t.Fatalf("could not open db: %v", err)
	}

	// create test table
	_, err = db.Exec(`CREATE VIRTUAL TABLE vt USING updatetest(f1 integer, f2 text, f3 text)`)
	if err != nil {
		t.Fatalf("could not create updatetest vtable vt, got: %v", err)
	}

	// check that table is defined properly
	if len(updateMod.tables) != 1 {
		t.Fatalf("expected exactly 1 table to exist, got: %d", len(updateMod.tables))
	}
	if _, ok := updateMod.tables["vt"]; !ok {
		t.Fatalf("expected table `vt` to exist in tables")
	}

	// check nothing in updatetest
	rows, err := db.Query(`select * from vt`)
	if err != nil {
		t.Fatalf("could not query vt, got: %v", err)
	}
	i, err := getRowCount(rows)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if i != 0 {
		t.Fatalf("expected no rows in vt, got: %d", i)
	}

	_, err = db.Exec(`delete from vt where f1 = 'yes'`)
	if err != nil {
		t.Fatalf("expected error on delete, got nil")
	}

	// test bad column name
	_, err = db.Exec(`insert into vt (f4) values('a')`)
	if err == nil {
		t.Fatalf("expected error on insert, got nil")
	}

	// insert to vt
	res, err := db.Exec(`insert into vt (f1, f2, f3) values (115, 'b', 'c'), (116, 'd', 'e')`)
	if err != nil {
		t.Fatalf("expected no error on insert, got: %v", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 1 row affected, got: %d", n)
	}

	// check vt table
	vt := updateMod.tables["vt"]
	if len(vt.data) != 2 {
		t.Fatalf("expected table vt to have exactly 2 rows, got: %d", len(vt.data))
	}
	if !reflect.DeepEqual(vt.data[0], []interface{}{int64(115), "b", "c"}) {
		t.Fatalf("expected table vt entry 0 to be [115 b c], instead: %v", vt.data[0])
	}
	if !reflect.DeepEqual(vt.data[1], []interface{}{int64(116), "d", "e"}) {
		t.Fatalf("expected table vt entry 1 to be [116 d e], instead: %v", vt.data[1])
	}

	// query vt
	var f1 int
	var f2, f3 string
	err = db.QueryRow(`select * from vt where f1 = 115`).Scan(&f1, &f2, &f3)
	if err != nil {
		t.Fatalf("expected no error on vt query, got: %v", err)
	}

	// check column values
	if f1 != 115 || f2 != "b" || f3 != "c" {
		t.Errorf("expected f1==115, f2==b, f3==c, got: %d, %q, %q", f1, f2, f3)
	}

	// update vt
	res, err = db.Exec(`update vt set f1=117, f2='f' where f3='e'`)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	n, err = res.RowsAffected()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly one row updated, got: %d", n)
	}

	// check vt table
	if len(vt.data) != 2 {
		t.Fatalf("expected table vt to have exactly 2 rows, got: %d", len(vt.data))
	}
	if !reflect.DeepEqual(vt.data[0], []interface{}{int64(115), "b", "c"}) {
		t.Fatalf("expected table vt entry 0 to be [115 b c], instead: %v", vt.data[0])
	}
	if !reflect.DeepEqual(vt.data[1], []interface{}{int64(117), "f", "e"}) {
		t.Fatalf("expected table vt entry 1 to be [117 f e], instead: %v", vt.data[1])
	}

	// delete from vt
	res, err = db.Exec(`delete from vt where f1 = 117`)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	n, err = res.RowsAffected()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly one row deleted, got: %d", n)
	}

	// check vt table
	if len(vt.data) != 1 {
		t.Fatalf("expected table vt to have exactly 1 row, got: %d", len(vt.data))
	}
	if !reflect.DeepEqual(vt.data[0], []interface{}{int64(115), "b", "c"}) {
		t.Fatalf("expected table vt entry 0 to be [115 b c], instead: %v", vt.data[0])
	}

	// check updatetest has 1 result
	rows, err = db.Query(`select * from vt`)
	if err != nil {
		t.Fatalf("could not query vt, got: %v", err)
	}
	i, err = getRowCount(rows)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if i != 1 {
		t.Fatalf("expected 1 row in vt, got: %d", i)
	}
}

func getRowCount(rows *sql.Rows) (int, error) {
	var i int
	for rows.Next() {
		i++
	}
	return i, nil
}

type vtabUpdateModule struct {
	t      *testing.T
	tables map[string]*vtabUpdateTable
}

func (m *vtabUpdateModule) Create(c *SQLiteConn, args []string) (VTab, error) {
	if len(args) < 2 {
		return nil, errors.New("must declare at least one column")
	}

	// get database name, table name, and column declarations ...
	dbname, tname, decls := args[1], args[2], args[3:]

	// extract column names + types from parameters declarations
	cols, typs := make([]string, len(decls)), make([]string, len(decls))
	for i := 0; i < len(decls); i++ {
		n, typ := decls[i], ""
		if j := strings.IndexAny(n, " \t\n"); j != -1 {
			typ, n = strings.TrimSpace(n[j+1:]), n[:j]
		}
		cols[i], typs[i] = n, typ
	}

	// declare table
	err := c.DeclareVTab(fmt.Sprintf(`CREATE TABLE "%s"."%s" (%s)`, dbname, tname, strings.Join(decls, ",")))
	if err != nil {
		return nil, err
	}

	// create table
	vtab := &vtabUpdateTable{m.t, dbname, tname, cols, typs, make([][]interface{}, 0)}
	m.tables[tname] = vtab
	return vtab, nil
}

func (m *vtabUpdateModule) Connect(c *SQLiteConn, args []string) (VTab, error) {
	return m.Create(c, args)
}

func (m *vtabUpdateModule) DestroyModule() {}

type vtabUpdateTable struct {
	t    *testing.T
	db   string
	name string
	cols []string
	typs []string
	data [][]interface{}
}

func (t *vtabUpdateTable) Open() (VTabCursor, error) {
	return &vtabUpdateCursor{t, 0}, nil
}

func (t *vtabUpdateTable) BestIndex(cst []InfoConstraint, ob []InfoOrderBy) (*IndexResult, error) {
	return &IndexResult{Used: make([]bool, len(cst))}, nil
}

func (t *vtabUpdateTable) Disconnect() error {
	return nil
}

func (t *vtabUpdateTable) Destroy() error {
	return nil
}

func (t *vtabUpdateTable) Insert(id interface{}, vals []interface{}) (int64, error) {
	var i int64
	if id == nil {
		i, t.data = int64(len(t.data)), append(t.data, vals)
		return i, nil
	}

	var ok bool
	i, ok = id.(int64)
	if !ok {
		return 0, fmt.Errorf("id is invalid type: %T", id)
	}

	t.data[i] = vals

	return i, nil
}

func (t *vtabUpdateTable) Update(id interface{}, vals []interface{}) error {
	i, ok := id.(int64)
	if !ok {
		return fmt.Errorf("id is invalid type: %T", id)
	}

	if int(i) >= len(t.data) || i < 0 {
		return fmt.Errorf("invalid row id %d", i)
	}

	t.data[int(i)] = vals

	return nil
}

func (t *vtabUpdateTable) Delete(id interface{}) error {
	i, ok := id.(int64)
	if !ok {
		return fmt.Errorf("id is invalid type: %T", id)
	}

	if int(i) >= len(t.data) || i < 0 {
		return fmt.Errorf("invalid row id %d", i)
	}

	t.data = append(t.data[:i], t.data[i+1:]...)

	return nil
}

type vtabUpdateCursor struct {
	t *vtabUpdateTable
	i int
}

func (c *vtabUpdateCursor) Column(ctxt *SQLiteContext, col int) error {
	switch x := c.t.data[c.i][col].(type) {
	case []byte:
		ctxt.ResultBlob(x)
	case bool:
		ctxt.ResultBool(x)
	case float64:
		ctxt.ResultDouble(x)
	case int:
		ctxt.ResultInt(x)
	case int64:
		ctxt.ResultInt64(x)
	case nil:
		ctxt.ResultNull()
	case string:
		ctxt.ResultText(x)
	default:
		ctxt.ResultText(fmt.Sprintf("%v", x))
	}

	return nil
}

func (c *vtabUpdateCursor) Filter(ixNum int, ixName string, vals []interface{}) error {
	return nil
}

func (c *vtabUpdateCursor) Next() error {
	c.i++
	return nil
}

func (c *vtabUpdateCursor) EOF() bool {
	return c.i >= len(c.t.data)
}

func (c *vtabUpdateCursor) Rowid() (int64, error) {
	return int64(c.i), nil
}

func (c *vtabUpdateCursor) Close() error {
	return nil
}
