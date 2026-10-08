// Package db opens the SQLite database used by WallWeave and applies the
// schema (tables) on startup.
package db

import (
	"database/sql"
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// schema holds the contents of schema.sql, embedded into the binary at
// compile time. It is executed (as a migration) every time the DB opens.
//
//go:embed schema.sql
var schema string

// Database is a thin wrapper around the SQL database handle so other
// packages can use the connection without importing "database/sql" details.
type Database struct {
	DB *sql.DB
}

// NewDatabase opens the default database file, which is "wallweave.db" in
// the current working directory.
func NewDatabase() (Database, error) {
	return Open("wallweave.db")
}

// Open opens (or creates) a SQLite database at the given file path.
// Steps:
//  1. Create the parent folder if the path contains one.
//  2. Open the SQLite file with the modernc driver.
//  3. Enable a 5 second busy timeout and WAL journal mode on EVERY pooled
//     connection (via the DSN), so concurrent access — including a second
//     wallweave process on another monitor — waits instead of failing with
//     "database is locked".
//  4. Run the embedded schema (create tables if they do not exist),
//     retrying briefly while the file is locked: creating a fresh WAL
//     database or recovering one after an unclean shutdown returns
//     SQLITE_BUSY right away, without honoring busy_timeout.
//  5. Return a Database that wraps the open connection.
func Open(path string) (Database, error) {
	// Step 1: make sure the parent directory exists.
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Database{}, err
		}
	}

	// Step 2: open the SQLite file (the driver was registered by the blank
	// import of modernc.org/sqlite above).
	// Step 3: the pragmas go in the DSN because database/sql opens several
	// connections and a PRAGMA run with Exec only reaches one of them.
	// busy_timeout comes first so switching to WAL already waits on locks.
	handle, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return Database{}, err
	}

	// Step 4: create the tables defined in schema.sql.
	deadline := time.Now().Add(5 * time.Second)
	for {
		err = migrate(handle)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "database is locked") || time.Now().After(deadline) {
			handle.Close()
			return Database{}, err
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Step 5: wrap the open handle and return it.
	return Database{DB: handle}, nil
}

// migrate runs the embedded schema SQL against the database. It is safe to
// run many times because the schema uses "IF NOT EXISTS".
func migrate(database *sql.DB) error {
	_, err := database.Exec(schema)
	return err
}

// Close closes the database connection. It does nothing when the handle is
// nil (for example if Open failed earlier).
func (d Database) Close() error {
	if d.DB == nil {
		return nil
	}
	return d.DB.Close()
}
