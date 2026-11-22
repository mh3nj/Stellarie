package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"stellarei/migrations"
)

// schemaMigrationsDDL bootstraps the bookkeeping table itself. It has to be
// applied outside the normal migration list because Migrate needs it to
// exist before it can even check what's already been applied.
const schemaMigrationsDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	filename   TEXT PRIMARY KEY,
	applied_at INTEGER NOT NULL
);`

// Migrate applies every embedded *.sql file that isn't already recorded in
// schema_migrations, in lexical filename order, each inside its own
// transaction. It is safe to call on every startup - a fully-migrated
// database is a no-op.
//
// This is intentionally up-only. STELLAREI must never destroy user data
// during a schema upgrade (section 46/85 #42), so there is no down/rollback
// path; a bad migration gets fixed by shipping a corrective forward
// migration, not by reverting.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schemaMigrationsDDL); err != nil {
		return fmt.Errorf("migrate: bootstrap schema_migrations: %w", err)
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("migrate: reading embedded migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files) // filenames are zero-padded, so lexical == numeric order

	applied, err := appliedMigrations(ctx, db)
	if err != nil {
		return err
	}

	for _, name := range files {
		if applied[name] {
			continue
		}
		if err := applyMigration(ctx, db, name); err != nil {
			return fmt.Errorf("migrate: applying %s: %w", name, err)
		}
	}

	return nil
}

func appliedMigrations(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT filename FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("migrate: reading applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

func applyMigration(ctx context.Context, db *sql.DB, name string) error {
	sqlBytes, err := migrations.FS.ReadFile(name)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (filename, applied_at) VALUES (?, unixepoch())`,
		name,
	); err != nil {
		return err
	}

	return tx.Commit()
}
