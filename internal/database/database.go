// Package database owns the single SQLite connection STELLAREI runs on and
// the migration runner that keeps its schema up to date.
//
// Deliberately CGO-free: it uses modernc.org/sqlite (a pure-Go SQLite
// driver) instead of mattn/go-sqlite3, so cross-compiling the Windows
// binary from any dev machine doesn't require a C toolchain or CGO=1.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Config controls how the connection is opened. Callers normally build this
// from config.Paths.Database plus values read out of the settings table
// (e.g. a user-relocated database path).
type Config struct {
	Path string // absolute path to the .db file
}

// Open opens (creating if necessary) the SQLite database at cfg.Path,
// applies the performance-critical PRAGMAs described in section 52, and
// runs any pending migrations.
//
// The returned *sql.DB is safe for concurrent use by multiple goroutines
// callers should NOT open more than one of these per process. SQLite in WAL
// mode tolerates concurrent readers plus a single writer just fine through
// Go's connection pool; the busy_timeout PRAGMA absorbs the rest.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("database: empty path")
	}

	// _pragma query params are applied by modernc.org/sqlite on every new
	// connection it opens for the pool, which matters because database/sql
	// silently opens more than one *physical* connection under load.
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)",
		cfg.Path,
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}

	// A single writer is the whole point of WAL-mode SQLite; capping the
	// pool avoids SQLITE_BUSY storms under write-heavy bursts (e.g. the
	// Activity Engine flushing a batch while Analytics is querying).
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(0)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database: migrate: %w", err)
	}

	return db, nil
}
