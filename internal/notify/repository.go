// Package notify implements STELLAREI's notification history (the
// `notifications` table - spec sections 15/19/22) and the native
// OS-notification dispatch that accompanies every alarm/timer/budget
// event worth surfacing. Named "notify" rather than "notifications" so
// it never collides with (or gets confused for) Wails's own
// github.com/wailsapp/wails/v3/pkg/services/notifications package, which
// dispatcher.go wraps - see that file's doc comment.
package notify

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Source is notifications.source - what kind of thing triggered this
// notification, matching the table's own comment.
type Source string

const (
	SourceAlarm  Source = "alarm"
	SourceBudget Source = "budget"
	SourceGoal   Source = "goal"
	// SourceSystem covers everything the schema's own comment doesn't
	// give a dedicated value to - notably timer completions, since
	// 0001_init_schema.sql's `source` comment only lists
	// alarm|budget|goal|system and adding a 5th value here would mean a
	// migration this phase doesn't otherwise need.
	SourceSystem Source = "system"
)

// Record mirrors a notifications row - the persisted history, separate
// from whether the native OS banner actually got shown (Dispatcher's
// job, which can fail independently - section 56: a notification
// subsystem hiccup must not stop tracking or break the rest of the app).
type Record struct {
	ID          int64
	Source      Source
	SourceID    int64
	Title       string
	Body        string
	CreatedAt   time.Time
	DismissedAt *time.Time
}

// Repository is the SQL-backed store for notification history.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Insert logs a notification that was (attempted to be) shown. Called
// regardless of whether Dispatcher.Send actually succeeded - the history
// list should show "STELLAREI tried to tell you this" even if the OS
// banner itself failed to render, since the underlying event (alarm
// fired, budget exceeded) still happened and still matters.
func (r *Repository) Insert(ctx context.Context, source Source, sourceID int64, title, body string) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO notifications (source, source_id, title, body, created_at)
		VALUES (?, ?, ?, ?, unixepoch())
	`, string(source), nullableID(sourceID), title, nullableString(body))
	if err != nil {
		return 0, fmt.Errorf("notify: inserting: %w", err)
	}
	return res.LastInsertId()
}

// ListRecent returns the most recent notifications, newest first - a
// simple in-app history (e.g. "what did I miss") independent of whatever
// the OS's own notification center shows.
func (r *Repository) ListRecent(ctx context.Context, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, source, COALESCE(source_id,0), title, COALESCE(body,''), created_at, dismissed_at
		FROM notifications ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("notify: listing: %w", err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var rec Record
		var source string
		var createdAtUnix int64
		var dismissedAtUnix sql.NullInt64
		if err := rows.Scan(&rec.ID, &source, &rec.SourceID, &rec.Title, &rec.Body, &createdAtUnix, &dismissedAtUnix); err != nil {
			return nil, fmt.Errorf("notify: scanning row: %w", err)
		}
		rec.Source = Source(source)
		rec.CreatedAt = time.Unix(createdAtUnix, 0)
		if dismissedAtUnix.Valid {
			when := time.Unix(dismissedAtUnix.Int64, 0)
			rec.DismissedAt = &when
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Dismiss marks a notification as read/acknowledged in the in-app
// history.
func (r *Repository) Dismiss(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE notifications SET dismissed_at = ? WHERE id = ?`, at.Unix(), id)
	if err != nil {
		return fmt.Errorf("notify: dismissing %d: %w", id, err)
	}
	return nil
}

func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
