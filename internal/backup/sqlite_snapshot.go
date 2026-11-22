package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
)

// CreateSnapshot writes a complete, consistent copy of the live database
// to destPath using SQLite's own `VACUUM INTO` - a single SQL statement
// (real, standard SQLite since 3.27.0, not a filesystem-level trick)
// that produces a correct snapshot even while the database is open in
// WAL mode with uncheckpointed writes sitting in the -wal file,
// verified against real SQLite before this wrapper was written (see
// this package's own doc comment). No separate locking or connection
// juggling needed - VACUUM INTO handles consistency internally.
//
// destPath must not already exist - SQLite's own VACUUM INTO refuses to
// overwrite an existing file (confirmed in the same verification pass),
// which this function surfaces as a clear error rather than a cryptic
// SQLite one, so callers that generate a fresh Filename() and still hit
// this know to retry with a different name rather than silently losing
// data to an overwrite.
func CreateSnapshot(ctx context.Context, db *sql.DB, destPath string) error {
	if destPath == "" {
		return fmt.Errorf("backup: destination path must not be empty")
	}
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("backup: %s already exists - VACUUM INTO never overwrites, generate a fresh filename", destPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("backup: checking destination: %w", err)
	}

	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, destPath); err != nil {
		return fmt.Errorf("backup: creating snapshot: %w", err)
	}
	return nil
}
