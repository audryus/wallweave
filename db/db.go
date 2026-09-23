// Package db opens the SQLite database used by WallWeave and applies the
// schema (tables) on startup.
package db

import (
	"database/sql"
	_ "embed"
	"os"
	"path/filepath"

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
//  3. Enable WAL journal mode and a 5 second busy timeout, so concurrent
//     reads do not fail with "database is locked".
//  4. Run the embedded schema (create tables if they do not exist).
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
	handle, err := sql.Open("sqlite", path)
	if err != nil {
		return Database{}, err
	}

	// Step 3: WAL + busy_timeout so concurrent reads do not hit
	// "database is locked".
	if _, err := handle.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		handle.Close()
		return Database{}, err
	}

	// Step 4: create the tables defined in schema.sql.
	if err := migrate(handle); err != nil {
		handle.Close()
		return Database{}, err
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
