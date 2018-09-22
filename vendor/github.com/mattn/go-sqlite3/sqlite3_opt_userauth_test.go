// Copyright (C) 2018 G.J.R. Timmer <gjr.timmer@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// +build sqlite_userauth

package sqlite3

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
)

var (
	conn             *SQLiteConn
	create           func(t *testing.T, username, password string) (file string, err error)
	createWithCrypt  func(t *testing.T, username, password, crypt, salt string) (file string, err error)
	connect          func(t *testing.T, f string, username, password string) (file string, db *sql.DB, c *SQLiteConn, err error)
	connectWithCrypt func(t *testing.T, f string, username, password string, crypt string, salt string) (file string, db *sql.DB, c *SQLiteConn, err error)
	authEnabled      func(db *sql.DB) (exists bool, err error)
	addUser          func(db *sql.DB, username, password string, admin int) (rv int, err error)
	userExists       func(db *sql.DB, username string) (rv int, err error)
	isAdmin          func(db *sql.DB, username string) (rv bool, err error)
	modifyUser       func(db *sql.DB, username, password string, admin int) (rv int, err error)
	deleteUser       func(db *sql.DB, username string) (rv int, err error)
)

func init() {
	// Create database connection
	sql.Register("sqlite3_with_conn",
		&SQLiteDriver{
			ConnectHook: func(c *SQLiteConn) error {
				conn = c
				return nil
			},
		})

	create = func(t *testing.T, username, password string) (file string, err error) {
		var db *sql.DB
		file, db, _, err = connect(t, "", username, password)
		db.Close()
		return
	}

	createWithCrypt = func(t *testing.T, username, password, crypt, salt string) (file string, err error) {
		var db *sql.DB
		file, db, _, err = connectWithCrypt(t, "", "admin", "admin", crypt, salt)
		db.Close()
		return
	}

	connect = func(t *testing.T, f string, username, password string) (file string, db *sql.DB, c *SQLiteConn, err error) {
		conn = nil // Clear connection
		file = f   // Copy provided file (f) => file
		if file == "" {
			// Create dummy file
			file = TempFilename(t)
		}

		db, err = sql.Open("sqlite3_with_conn", "file:"+file+fmt.Sprintf("?_auth&_auth_user=%s&_auth_pass=%s", username, password))
		if err != nil {
			defer os.Remove(file)
			return file, nil, nil, err
		}

		// Dummy query to force connection and database creation
		// Will return ErrUnauthorized (SQLITE_AUTH) if user authentication fails
		if _, err = db.Exec("SELECT 1;"); err != nil {
			defer os.Remove(file)
			defer db.Close()
			return file, nil, nil, err
		}
		c = conn

		return
	}

	connectWithCrypt = func(t *testing.T, f string, username, password string, crypt string, salt string) (file string, db *sql.DB, c *SQLiteConn, err error) {
		conn = nil // Clear connection
		file = f   // Copy provided file (f) => file
		if file == "" {
			// Create dummy file
			file = TempFilename(t)
		}

		db, err = sql.Open("sqlite3_with_conn", "file:"+file+fmt.Sprintf("?_auth&_auth_user=%s&_auth_pass=%s&_auth_crypt=%s&_auth_salt=%s", username, password, crypt, salt))
		if err != nil {
			defer os.Remove(file)
			return file, nil, nil, err
		}

		// Dummy query to force connection and database creation
		// Will return ErrUnauthorized (SQLITE_AUTH) if user authentication fails
		if _, err = db.Exec("SELECT 1;"); err != nil {
			defer os.Remove(file)
			defer db.Close()
			return file, nil, nil, err
		}
		c = conn

		return
	}

	authEnabled = func(db *sql.DB) (exists bool, err error) {
		err = db.QueryRow("select count(type) from sqlite_master WHERE type='table' and name='sqlite_user';").Scan(&exists)
		return
	}

	addUser = func(db *sql.DB, username, password string, admin int) (rv int, err error) {
		err = db.QueryRow("select auth_user_add(?, ?, ?);", username, password, admin).Scan(&rv)
		return
	}

	userExists = func(db *sql.DB, username string) (rv int, err error) {
		err = db.QueryRow("select count(uname) from sqlite_user where uname=?", username).Scan(&rv)
		return
	}

	isAdmin = func(db *sql.DB, username string) (rv bool, err error) {
		err = db.QueryRow("select isAdmin from sqlite_user where uname=?", username).Scan(&rv)
		return
	}

	modifyUser = func(db *sql.DB, username, password string, admin int) (rv int, err error) {
		err = db.QueryRow("select auth_user_change(?, ?, ?);", username, password, admin).Scan(&rv)
		return
	}

	deleteUser = func(db *sql.DB, username string) (rv int, err error) {
		err = db.QueryRow("select auth_user_delete(?);", username).Scan(&rv)
		return
	}
}

func TestUserAuthCreateDatabase(t *testing.T) {
	f, db, c, err := connect(t, "", "admin", "admin")
	if err != nil && c == nil && db == nil {
		t.Fatal(err)
	}
	defer db.Close()
	defer os.Remove(f)

	enabled, err := authEnabled(db)
	if err != nil || !enabled {
		t.Fatalf("UserAuth not enabled: %s", err)
	}

	e, err := userExists(db, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if e != 1 {
		t.Fatal("UserAuth: admin does not exists")
	}
	a, err := isAdmin(db, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if !a {
		t.Fatal("UserAuth: User is not administrator")
	}
}

func TestUserAuthLogin(t *testing.T) {
	f1, err := create(t, "admin", "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f1)

	f2, db2, c2, err := connect(t, f1, "admin", "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if f1 != f2 {
		t.Fatal("UserAuth: Database file mismatch")
	}

	// Test lower level authentication
	err = c2.Authenticate("admin", "admin")
	if err != nil {
		t.Fatalf("UserAuth: *SQLiteConn.Authenticate() Failed: %s", err)
	}

	// Test Login Failed
	_, _, _, err = connect(t, f1, "admin", "invalid")
	if err == nil {
		t.Fatal("Login successful while expecting to fail")
	}
	if err != ErrUnauthorized {
		t.Fatal(err)
	}
	err = c2.Authenticate("admin", "invalid")
	if err == nil {
		t.Fatal("Login successful while expecting to fail")
	}
	if err != ErrUnauthorized {
		t.Fatal(err)
	}
}

func TestUserAuthAddAdmin(t *testing.T) {
	f, db, c, err := connect(t, "", "admin", "admin")
	if err != nil && c == nil && db == nil {
		t.Fatal(err)
	}
	defer db.Close()
	defer os.Remove(f)

	// Add Admin User through SQL call
	rv, err := addUser(db, "admin2", "admin2", 1)
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to add user")
	}

	// Check if user was created
	exists, err := userExists(db, "admin2")
	if err != nil {
		t.Fatal(err)
	}
	if exists != 1 {
		t.Fatal("UserAuth: 'admin2' does not exists")
	}

	// Check if user was created as an Administrator
	admin, err := isAdmin(db, "admin2")
	if err != nil {
		t.Fatal(err)
	}
	if !admin {
		t.Fatal("UserAuth: 'admin2' is not administrator")
	}

	// Test *SQLiteConn
	err = c.AuthUserAdd("admin3", "admin3", true)
	if err != nil {
		t.Fatal(err)
	}

	// Check if user was created
	exists, err = userExists(db, "admin2")
	if err != nil {
		t.Fatal(err)
	}
	if exists != 1 {
		t.Fatal("UserAuth: 'admin3' does not exists")
	}

	// Check if the user was created as an Administrator
	admin, err = isAdmin(db, "admin3")
	if err != nil {
		t.Fatal(err)
	}
	if !admin {
		t.Fatal("UserAuth: 'admin3' is not administrator")
	}
}

func TestUserAuthAddUser(t *testing.T) {
	f1, db1, c, err := connect(t, "", "admin", "admin")
	if err != nil && c == nil && db == nil {
		t.Fatal(err)
	}
	defer os.Remove(f1)

	// Add user through SQL call
	rv, err := addUser(db1, "user", "user", 0)
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to add user")
	}

	// Check if user was created
	exists, err := userExists(db1, "user")
	if err != nil {
		t.Fatal(err)
	}
	if exists != 1 {
		t.Fatal("UserAuth: 'user' does not exists")
	}

	// Check if user was created as an Administrator
	admin, err := isAdmin(db1, "user")
	if err != nil {
		t.Fatal(err)
	}
	if admin {
		t.Fatal("UserAuth: 'user' is administrator")
	}

	// Test *SQLiteConn
	err = c.AuthUserAdd("user2", "user2", false)
	if err != nil {
		t.Fatal(err)
	}

	// Check if user was created
	exists, err = userExists(db1, "user2")
	if err != nil {
		t.Fatal(err)
	}
	if exists != 1 {
		t.Fatal("UserAuth: 'user2' does not exists")
	}

	// Check if the user was created as an Administrator
	admin, err = isAdmin(db1, "user2")
	if err != nil {
		t.Fatal(err)
	}
	if admin {
		t.Fatal("UserAuth: 'user2' is administrator")
	}

	// Reconnect as normal user
	db1.Close()
	_, db2, c2, err := connect(t, f1, "user", "user")
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	// Try to create admin user while logged in as normal user
	rv, err = addUser(db2, "admin2", "admin2", 1)
	if err != nil {
		t.Fatal(err)
	}
	if rv != SQLITE_AUTH {
		t.Fatal("Created admin user while not allowed")
	}

	err = c2.AuthUserAdd("admin3", "admin3", true)
	if err != ErrAdminRequired {
		t.Fatal("Created admin user while not allowed")
	}

	// Try to create normal user while logged in as normal user
	rv, err = addUser(db2, "user3", "user3", 0)
	if err != nil {
		t.Fatal(err)
	}
	if rv != SQLITE_AUTH {
		t.Fatal("Created user while not allowed")
	}

	err = c2.AuthUserAdd("user4", "user4", false)
	if err != ErrAdminRequired {
		t.Fatal("Created user while not allowed")
	}
}

func TestUserAuthModifyUser(t *testing.T) {
	f1, db1, c1, err := connect(t, "", "admin", "admin")
	if err != nil && c1 == nil && db == nil {
		t.Fatal(err)
	}
	defer os.Remove(f1)

	// Modify Password for current logged in admin
	// through SQL
	rv, err := modifyUser(db1, "admin", "admin2", 1)
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to modify password for admin")
	}

	// Modify password for current logged in admin
	// through *SQLiteConn
	err = c1.AuthUserChange("admin", "admin3", true)
	if err != nil {
		t.Fatal(err)
	}

	// Modify Administrator Flag
	// Because we are current logged in as 'admin'
	// Changing our own admin flag should fail.
	rv, err = modifyUser(db1, "admin", "admin3", 0)
	if err != nil {
		t.Fatal(err)
	}
	if rv != SQLITE_AUTH {
		t.Fatal("Successfully changed admin flag while not allowed")
	}

	// Modify admin flag through (*SQLiteConn)
	// Because we are current logged in as 'admin'
	// Changing our own admin flag should fail.
	err = c1.AuthUserChange("admin", "admin3", false)
	if err != ErrAdminRequired {
		t.Fatal("Successfully changed admin flag while not allowed")
	}

	// Add normal user
	rv, err = addUser(db1, "user", "password", 0)
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to add user")
	}

	rv, err = addUser(db1, "user2", "user2", 0)
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to add user")
	}

	// Modify other user password and flag through SQL
	rv, err = modifyUser(db1, "user", "pass", 1)
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to modify password for user")
	}

	// Modify other user password and flag through *SQLiteConn
	err = c1.AuthUserChange("user", "newpass", false)
	if err != nil {
		t.Fatal(err)
	}

	// Disconnect database for reconnect
	db1.Close()
	_, db2, c2, err := connect(t, f1, "user", "newpass")
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	// Modify other user password through SQL
	rv, err = modifyUser(db2, "user2", "newpass", 0)
	if err != nil {
		t.Fatal(err)
	}
	if rv != SQLITE_AUTH {
		t.Fatal("Password change succesful while not allowed")
	}

	// Modify other user password and flag through *SQLiteConn
	err = c2.AuthUserChange("user2", "invalid", false)
	if err != ErrAdminRequired {
		t.Fatal("Password change succesful while not allowed")
	}
}

func TestUserAuthDeleteUser(t *testing.T) {
	f1, db1, c, err := connect(t, "", "admin", "admin")
	if err != nil && c == nil && db == nil {
		t.Fatal(err)
	}
	defer os.Remove(f1)

	// Add Admin User 2
	rv, err := addUser(db1, "admin2", "admin2", 1)
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to add user")
	}

	rv, err = addUser(db1, "admin3", "admin3", 1)
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to add user")
	}

	// Check if user was created
	exists, err := userExists(db1, "admin2")
	if err != nil {
		t.Fatal(err)
	}
	if exists != 1 {
		t.Fatal("UserAuth: 'admin2' does not exists")
	}

	exists, err = userExists(db1, "admin3")
	if err != nil {
		t.Fatal(err)
	}
	if exists != 1 {
		t.Fatal("UserAuth: 'admin2' does not exists")
	}

	// Delete user through SQL
	rv, err = deleteUser(db1, "admin2")
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to delete admin2")
	}

	// Verify user admin2 deleted
	exists, err = userExists(db1, "admin2")
	if err != nil {
		t.Fatal(err)
	}
	if exists != 0 {
		t.Fatal("UserAuth: 'admin2' still exists")
	}

	// Delete user through *SQLiteConn
	rv, err = deleteUser(db1, "admin3")
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to delete admin3")
	}

	// Verify user admin3 deleted
	exists, err = userExists(db1, "admin3")
	if err != nil {
		t.Fatal(err)
	}
	if exists != 0 {
		t.Fatal("UserAuth: 'admin3' still exists")
	}

	// Add normal user for reconnect and privileges check
	rv, err = addUser(db1, "reconnect", "reconnect", 0)
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to add user")
	}

	// Add normal user for deletion through SQL
	rv, err = addUser(db1, "user", "user", 0)
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to add user")
	}

	rv, err = addUser(db1, "user2", "user2", 0)
	if err != nil {
		t.Fatal(err)
	}
	if rv != 0 {
		t.Fatal("Failed to add user")
	}

	// Close database for reconnect
	db1.Close()

	// Reconnect as normal user
	_, db2, c2, err := connect(t, f1, "reconnect", "reconnect")
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	// Delete user while logged in as normal user
	// through SQL
	rv, err = deleteUser(db2, "user")
	if err != nil {
		t.Fatal(err)
	}
	if rv != SQLITE_AUTH {
		t.Fatal("Successfully deleted user wthout proper privileges")
	}

	// Delete user while logged in as normal user
	// through *SQLiteConn
	err = c2.AuthUserDelete("user2")
	if err != ErrAdminRequired {
		t.Fatal("Successfully deleted user wthout proper privileges")
	}
}

func TestUserAuthEncoders(t *testing.T) {
	cases := map[string]string{
		"sha1":    "",
		"ssha1":   "salted",
		"sha256":  "",
		"ssha256": "salted",
		"sha384":  "",
		"ssha384": "salted",
		"sha512":  "",
		"ssha512": "salted",
	}

	for enc, salt := range cases {
		f, err := createWithCrypt(t, "admin", "admin", enc, salt)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f)

		_, db, _, err := connectWithCrypt(t, f, "admin", "admin", enc, salt)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if e, err := authEnabled(db); err != nil && !e {
			t.Fatalf("UserAuth (%s) not enabled %s", enc, err)
		}
	}
}
