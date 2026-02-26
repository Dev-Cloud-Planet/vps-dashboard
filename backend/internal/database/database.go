package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open creates or opens the SQLite database at the given path and configures
// it with WAL mode and performance-oriented pragmas. The parent directory is
// created automatically if it does not exist.
func Open(dbPath string) (*sql.DB, error) {
	// Ensure the parent directory exists.
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("database: create directory %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("database: open %s: %w", dbPath, err)
	}

	// Apply performance and safety pragmas.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA temp_store=MEMORY",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("database: pragma %q: %w", p, err)
		}
	}

	// Verify the database is accessible.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	return db, nil
}
