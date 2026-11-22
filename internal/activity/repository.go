package activity

import (
	"context"
	"database/sql"
	"fmt"
)

// Repository is the Activity Engine's only path to the database. Kept as
// an interface so the Engine's poll-loop wiring can be tested against a
// fake without a real *sql.DB - see engine_test.go.
type Repository interface {
	// ResolveApplicationID finds the applications row for processName,
	// creating one if this is the first time it's been seen, and always
	// bumping last_seen_at. Returns the row's id.
	ResolveApplicationID(ctx context.Context, processName string) (int64, error)

	// InsertEvent persists one completed, debounced activity event tied
	// to applicationID. Returns the new row's id (needed by the
	// sessionizer, which references activity_events.id via
	// session_events).
	InsertEvent(ctx context.Context, applicationID int64, ev CompletedEvent) (int64, error)
}

type sqlRepository struct {
	db *sql.DB
}

// NewRepository wraps db (already open and migrated - see
// internal/database.Open) as a Repository.
func NewRepository(db *sql.DB) Repository {
	return &sqlRepository{db: db}
}

func (r *sqlRepository) ResolveApplicationID(ctx context.Context, processName string) (int64, error) {
	if processName == "" {
		processName = "(unknown)"
	}

	// Single upsert: applications.process_name is UNIQUE (schema
	// migration 0001), so ON CONFLICT is the atomic find-or-create this
	// needs without a separate SELECT-then-INSERT race.
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO applications (process_name, display_name, first_seen_at, last_seen_at)
		VALUES (?, ?, unixepoch(), unixepoch())
		ON CONFLICT(process_name) DO UPDATE SET last_seen_at = unixepoch()
	`, processName, processName)
	if err != nil {
		return 0, fmt.Errorf("activity: upserting application %q: %w", processName, err)
	}

	var id int64
	err = r.db.QueryRowContext(ctx,
		`SELECT id FROM applications WHERE process_name = ?`, processName,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("activity: resolving application id for %q: %w", processName, err)
	}
	return id, nil
}

func (r *sqlRepository) InsertEvent(ctx context.Context, applicationID int64, ev CompletedEvent) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO activity_events
			(started_at, duration_secs, application_id, window_title, idle, confidence, source)
		VALUES (?, ?, ?, ?, ?, 1.0, 'activity_engine')
	`,
		ev.StartedAt.Unix(),
		int64(ev.Duration.Seconds()),
		applicationID,
		ev.WindowTitle,
		boolToInt(ev.Idle),
	)
	if err != nil {
		return 0, fmt.Errorf("activity: inserting event: %w", err)
	}
	return res.LastInsertId()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
