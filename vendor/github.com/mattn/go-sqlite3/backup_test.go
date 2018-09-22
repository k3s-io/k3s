// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package sqlite3

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

// The number of rows of test data to create in the source database.
// Can be used to control how many pages are available to be backed up.
const testRowCount = 100

// The maximum number of seconds after which the page-by-page backup is considered to have taken too long.
const usePagePerStepsTimeoutSeconds = 30

// Test the backup functionality.
func testBackup(t *testing.T, testRowCount int, usePerPageSteps bool) {
	// This function will be called multiple times.
	// It uses sql.Register(), which requires the name parameter value to be unique.
	// There does not currently appear to be a way to unregister a registered driver, however.
	// So generate a database driver name that will likely be unique.
	var driverName = fmt.Sprintf("sqlite3_testBackup_%v_%v_%v", testRowCount, usePerPageSteps, time.Now().UnixNano())

	// The driver's connection will be needed in order to perform the backup.
	driverConns := []*SQLiteConn{}
	sql.Register(driverName, &SQLiteDriver{
		ConnectHook: func(conn *SQLiteConn) error {
			driverConns = append(driverConns, conn)
			return nil
		},
	})

	// Connect to the source database.
	srcTempFilename := TempFilename(t)
	defer os.Remove(srcTempFilename)
	srcDb, err := sql.Open(driverName, srcTempFilename)
	if err != nil {
		t.Fatal("Failed to open the source database:", err)
	}
	defer srcDb.Close()
	err = srcDb.Ping()
	if err != nil {
		t.Fatal("Failed to connect to the source database:", err)
	}

	// Connect to the destination database.
	destTempFilename := TempFilename(t)
	defer os.Remove(destTempFilename)
	destDb, err := sql.Open(driverName, destTempFilename)
	if err != nil {
		t.Fatal("Failed to open the destination database:", err)
	}
	defer destDb.Close()
	err = destDb.Ping()
	if err != nil {
		t.Fatal("Failed to connect to the destination database:", err)
	}

	// Check the driver connections.
	if len(driverConns) != 2 {
		t.Fatalf("Expected 2 driver connections, but found %v.", len(driverConns))
	}
	srcDbDriverConn := driverConns[0]
	if srcDbDriverConn == nil {
		t.Fatal("The source database driver connection is nil.")
	}
	destDbDriverConn := driverConns[1]
	if destDbDriverConn == nil {
		t.Fatal("The destination database driver connection is nil.")
	}

	// Generate some test data for the given ID.
	var generateTestData = func(id int) string {
		return fmt.Sprintf("test-%v", id)
	}

	// Populate the source database with a test table containing some test data.
	tx, err := srcDb.Begin()
	if err != nil {
		t.Fatal("Failed to begin a transaction when populating the source database:", err)
	}
	_, err = srcDb.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)")
	if err != nil {
		tx.Rollback()
		t.Fatal("Failed to create the source database \"test\" table:", err)
	}
	for id := 0; id < testRowCount; id++ {
		_, err = srcDb.Exec("INSERT INTO test (id, value) VALUES (?, ?)", id, generateTestData(id))
		if err != nil {
			tx.Rollback()
			t.Fatal("Failed to insert a row into the source database \"test\" table:", err)
		}
	}
	err = tx.Commit()
	if err != nil {
		t.Fatal("Failed to populate the source database:", err)
	}

	// Confirm that the destination database is initially empty.
	var destTableCount int
	err = destDb.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'").Scan(&destTableCount)
	if err != nil {
		t.Fatal("Failed to check the destination table count:", err)
	}
	if destTableCount != 0 {
		t.Fatalf("The destination database is not empty; %v table(s) found.", destTableCount)
	}

	// Prepare to perform the backup.
	backup, err := destDbDriverConn.Backup("main", srcDbDriverConn, "main")
	if err != nil {
		t.Fatal("Failed to initialize the backup:", err)
	}

	// Allow the initial page count and remaining values to be retrieved.
	// According to <https://www.sqlite.org/c3ref/backup_finish.html>, the page count and remaining values are "... only updated by sqlite3_backup_step()."
	isDone, err := backup.Step(0)
	if err != nil {
		t.Fatal("Unable to perform an initial 0-page backup step:", err)
	}
	if isDone {
		t.Fatal("Backup is unexpectedly done.")
	}

	// Check that the page count and remaining values are reasonable.
	initialPageCount := backup.PageCount()
	if initialPageCount <= 0 {
		t.Fatalf("Unexpected initial page count value: %v", initialPageCount)
	}
	initialRemaining := backup.Remaining()
	if initialRemaining <= 0 {
		t.Fatalf("Unexpected initial remaining value: %v", initialRemaining)
	}
	if initialRemaining != initialPageCount {
		t.Fatalf("Initial remaining value differs from the initial page count value; remaining: %v; page count: %v", initialRemaining, initialPageCount)
	}

	// Perform the backup.
	if usePerPageSteps {
		var startTime = time.Now().Unix()

		// Test backing-up using a page-by-page approach.
		var latestRemaining = initialRemaining
		for {
			// Perform the backup step.
			isDone, err = backup.Step(1)
			if err != nil {
				t.Fatal("Failed to perform a backup step:", err)
			}

			// The page count should remain unchanged from its initial value.
			currentPageCount := backup.PageCount()
			if currentPageCount != initialPageCount {
				t.Fatalf("Current page count differs from the initial page count; initial page count: %v; current page count: %v", initialPageCount, currentPageCount)
			}

			// There should now be one less page remaining.
			currentRemaining := backup.Remaining()
			expectedRemaining := latestRemaining - 1
			if currentRemaining != expectedRemaining {
				t.Fatalf("Unexpected remaining value; expected remaining value: %v; actual remaining value: %v", expectedRemaining, currentRemaining)
			}
			latestRemaining = currentRemaining

			if isDone {
				break
			}

			// Limit the runtime of the backup attempt.
			if (time.Now().Unix() - startTime) > usePagePerStepsTimeoutSeconds {
				t.Fatal("Backup is taking longer than expected.")
			}
		}
	} else {
		// Test the copying of all remaining pages.
		isDone, err = backup.Step(-1)
		if err != nil {
			t.Fatal("Failed to perform a backup step:", err)
		}
		if !isDone {
			t.Fatal("Backup is unexpectedly not done.")
		}
	}

	// Check that the page count and remaining values are reasonable.
	finalPageCount := backup.PageCount()
	if finalPageCount != initialPageCount {
		t.Fatalf("Final page count differs from the initial page count; initial page count: %v; final page count: %v", initialPageCount, finalPageCount)
	}
	finalRemaining := backup.Remaining()
	if finalRemaining != 0 {
		t.Fatalf("Unexpected remaining value: %v", finalRemaining)
	}

	// Finish the backup.
	err = backup.Finish()
	if err != nil {
		t.Fatal("Failed to finish backup:", err)
	}

	// Confirm that the "test" table now exists in the destination database.
	var doesTestTableExist bool
	err = destDb.QueryRow("SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'test' LIMIT 1) AS test_table_exists").Scan(&doesTestTableExist)
	if err != nil {
		t.Fatal("Failed to check if the \"test\" table exists in the destination database:", err)
	}
	if !doesTestTableExist {
		t.Fatal("The \"test\" table could not be found in the destination database.")
	}

	// Confirm that the number of rows in the destination database's "test" table matches that of the source table.
	var actualTestTableRowCount int
	err = destDb.QueryRow("SELECT COUNT(*) FROM test").Scan(&actualTestTableRowCount)
	if err != nil {
		t.Fatal("Failed to determine the rowcount of the \"test\" table in the destination database:", err)
	}
	if testRowCount != actualTestTableRowCount {
		t.Fatalf("Unexpected destination \"test\" table row count; expected: %v; found: %v", testRowCount, actualTestTableRowCount)
	}

	// Check each of the rows in the destination database.
	for id := 0; id < testRowCount; id++ {
		var checkedValue string
		err = destDb.QueryRow("SELECT value FROM test WHERE id = ?", id).Scan(&checkedValue)
		if err != nil {
			t.Fatal("Failed to query the \"test\" table in the destination database:", err)
		}

		var expectedValue = generateTestData(id)
		if checkedValue != expectedValue {
			t.Fatalf("Unexpected value in the \"test\" table in the destination database; expected value: %v; actual value: %v", expectedValue, checkedValue)
		}
	}
}

func TestBackupStepByStep(t *testing.T) {
	testBackup(t, testRowCount, true)
}

func TestBackupAllRemainingPages(t *testing.T) {
	testBackup(t, testRowCount, false)
}

// Test the error reporting when preparing to perform a backup.
func TestBackupError(t *testing.T) {
	const driverName = "sqlite3_TestBackupError"

	// The driver's connection will be needed in order to perform the backup.
	var dbDriverConn *SQLiteConn
	sql.Register(driverName, &SQLiteDriver{
		ConnectHook: func(conn *SQLiteConn) error {
			dbDriverConn = conn
			return nil
		},
	})

	// Connect to the database.
	dbTempFilename := TempFilename(t)
	defer os.Remove(dbTempFilename)
	db, err := sql.Open(driverName, dbTempFilename)
	if err != nil {
		t.Fatal("Failed to open the database:", err)
	}
	defer db.Close()
	db.Ping()

	// Need the driver connection in order to perform the backup.
	if dbDriverConn == nil {
		t.Fatal("Failed to get the driver connection.")
	}

	// Prepare to perform the backup.
	// Intentionally using the same connection for both the source and destination databases, to trigger an error result.
	backup, err := dbDriverConn.Backup("main", dbDriverConn, "main")
	if err == nil {
		t.Fatal("Failed to get the expected error result.")
	}
	const expectedError = "source and destination must be distinct"
	if err.Error() != expectedError {
		t.Fatalf("Unexpected error message; expected value: \"%v\"; actual value: \"%v\"", expectedError, err.Error())
	}
	if backup != nil {
		t.Fatal("Failed to get the expected nil backup result.")
	}
}
