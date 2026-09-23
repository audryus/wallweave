package db

import (
	"database/sql"
	_ "embed"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type Database struct {
	DB *sql.DB
}

func NewDatabase() (Database, error) {
	return Open("wallweave.db")
}

func Open(path string) (Database, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Database{}, err
		}
	}

	handle, err := sql.Open("sqlite", path)
	if err != nil {
		return Database{}, err
	}

	// WAL + busy_timeout: leituras concorrentes sem "database is locked"
	if _, err := handle.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		handle.Close()
		return Database{}, err
	}

	if err := migrate(handle); err != nil {
		handle.Close()
		return Database{}, err
	}

	return Database{DB: handle}, nil
}

func migrate(database *sql.DB) error {
	_, err := database.Exec(schema)
	return err
}

func (d Database) Close() error {
	if d.DB == nil {
		return nil
	}
	return d.DB.Close()
}
