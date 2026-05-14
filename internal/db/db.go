// Package db provides database connection and migrations for GSLB.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// NewDB creates a new database connection with foreign keys, WAL mode, and a
// busy timeout enabled.
//
// WAL mode allows multiple concurrent readers alongside a single writer, so
// read-heavy API handlers are never blocked by an ongoing write.
//
// busy_timeout tells SQLite to retry automatically for up to 5 seconds when a
// write finds the database locked by another writer (SQLITE_BUSY), rather than
// returning an error immediately. This is sufficient to absorb contention from
// concurrent goroutines (reconciler, health-state processor, aggregator flush,
// cleanup loops) without serialising reads.
func NewDB(dsn string) (*sql.DB, error) {
	pragmaDSN := dsn + "?_pragma=foreign_keys(1)&_pragma=journal_mode=WAL&_pragma=busy_timeout(5000)"

	db, err := sql.Open("sqlite", pragmaDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
