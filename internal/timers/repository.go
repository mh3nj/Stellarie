package timers

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository is the SQL-backed store for timers rows. A concrete struct
// (not an interface), matching internal/budgets and internal/goals
// nothing needs to fake the whole repository in tests, since the
// interesting logic (Elapsed/Remaining/NextPhase) is pure and tested
// separately above. internal/scheduler defines its own narrow interface
// for the handful of methods IT needs, the same way it defines one for
// alarms - see internal/scheduler's doc comment.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func scanTimer(row interface{ Scan(...any) error }) (Timer, error) {
	var t Timer
	var timerType, status string
	var projectID, durationSecs sql.NullInt64
	var endedAtUnix, pausedAtUnix sql.NullInt64
	var startedAtUnix int64
	var label, metadata sql.NullString

	if err := row.Scan(&t.ID, &timerType, &projectID, &label, &durationSecs,
		&startedAtUnix, &endedAtUnix, &status, &metadata, &pausedAtUnix, &t.AccumulatedPauseSecs); err != nil {
		return Timer{}, err
	}

	t.TimerType = Type(timerType)
	t.Status = Status(status)
	t.ProjectID = projectID.Int64
	t.Label = label.String
	t.DurationSecs = durationSecs.Int64
	t.StartedAt = time.Unix(startedAtUnix, 0)
	t.MetadataJSON = metadata.String
	if endedAtUnix.Valid {
		when := time.Unix(endedAtUnix.Int64, 0)
		t.EndedAt = &when
	}
	if pausedAtUnix.Valid {
		when := time.Unix(pausedAtUnix.Int64, 0)
		t.PausedAt = &when
	}
	return t, nil
}

const selectColumns = `
	id, timer_type, project_id, label, duration_secs,
	started_at, ended_at, status, metadata_json, paused_at, accumulated_pause_secs
`

// Create starts a new timer, running immediately (section 21: timers
// start running, they don't sit in a "created but not started" limbo
// that's what Cancel is for if a person changes their mind).
// durationSecs <= 0 means no target (a stopwatch). metadataJSON is
// normally "" except for a Pomodoro timer's phase state (see
// pomodoro.go) - validated for well-formedness by the caller
// (TimerService), not here, since this layer shouldn't need to know
// Pomodoro's JSON shape.
func (r *Repository) Create(ctx context.Context, timerType Type, projectID int64, label string, durationSecs int64, metadataJSON string, startedAt time.Time) (int64, error) {
	if !ValidType(string(timerType)) {
		return 0, fmt.Errorf("timers: invalid timer type %q", timerType)
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO timers (timer_type, project_id, label, duration_secs, started_at, status, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, string(timerType), nullableID(projectID), nullableString(label), nullableDuration(durationSecs), startedAt.Unix(), string(StatusRunning), nullableString(metadataJSON))
	if err != nil {
		return 0, fmt.Errorf("timers: creating: %w", err)
	}
	return res.LastInsertId()
}

// Get returns a single timer by id.
func (r *Repository) Get(ctx context.Context, id int64) (Timer, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+selectColumns+` FROM timers WHERE id = ?`, id)
	t, err := scanTimer(row)
	if err == sql.ErrNoRows {
		return Timer{}, fmt.Errorf("timers: %d not found", id)
	}
	if err != nil {
		return Timer{}, fmt.Errorf("timers: get %d: %w", id, err)
	}
	return t, nil
}

// ListActive returns every running or paused timer - what the Timers
// page and the tray need to show "what's going right now", newest first
// so a just-started timer appears at the top.
func (r *Repository) ListActive(ctx context.Context) ([]Timer, error) {
	return r.query(ctx, `
		SELECT `+selectColumns+` FROM timers
		WHERE status IN ('running','paused')
		ORDER BY started_at DESC
	`)
}

// ListRecent returns the most recently completed/cancelled timers
// (history - e.g. "3 Pomodoros today"), most recent first.
func (r *Repository) ListRecent(ctx context.Context, limit int) ([]Timer, error) {
	if limit <= 0 {
		limit = 20
	}
	return r.query(ctx, `
		SELECT `+selectColumns+` FROM timers
		WHERE status IN ('completed','cancelled')
		ORDER BY started_at DESC
		LIMIT ?
	`, limit)
}

func (r *Repository) query(ctx context.Context, query string, args ...any) ([]Timer, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("timers: querying: %w", err)
	}
	defer rows.Close()

	var out []Timer
	for rows.Next() {
		t, err := scanTimer(rows)
		if err != nil {
			return nil, fmt.Errorf("timers: scanning row: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Pause marks a running timer paused at `at`. A no-op error (rather than
// silently doing nothing) if the timer isn't currently running, so the
// UI/tray can tell the difference between "paused" and "that timer
// doesn't exist / already isn't running".
func (r *Repository) Pause(ctx context.Context, id int64, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE timers SET status = ?, paused_at = ?
		WHERE id = ? AND status = ?
	`, string(StatusPaused), at.Unix(), id, string(StatusRunning))
	if err != nil {
		return fmt.Errorf("timers: pausing %d: %w", id, err)
	}
	return requireAffected(res, "timers: %d is not currently running", id)
}

// Resume clears a pause, folding the just-finished pause interval into
// accumulated_pause_secs so Elapsed keeps being correct going forward.
func (r *Repository) Resume(ctx context.Context, id int64, at time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("timers: resuming %d: %w", id, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	var pausedAtUnix sql.NullInt64
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT paused_at, status FROM timers WHERE id = ?`, id).Scan(&pausedAtUnix, &status); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("timers: %d not found", id)
		}
		return fmt.Errorf("timers: resuming %d: %w", id, err)
	}
	if status != string(StatusPaused) || !pausedAtUnix.Valid {
		return fmt.Errorf("timers: %d is not currently paused", id)
	}

	pauseDurationSecs := at.Unix() - pausedAtUnix.Int64
	if pauseDurationSecs < 0 {
		pauseDurationSecs = 0 // defensive: a clock that moved backward must not shrink accumulated time
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE timers
		SET status = ?, paused_at = NULL, accumulated_pause_secs = accumulated_pause_secs + ?
		WHERE id = ?
	`, string(StatusRunning), pauseDurationSecs, id); err != nil {
		return fmt.Errorf("timers: resuming %d: %w", id, err)
	}

	return tx.Commit()
}

// Complete marks a timer finished at `at` - called by both a person
// clicking "Finish" and internal/scheduler noticing a countdown reached
// its target. Idempotent-ish: completing an already-completed timer is a
// no-op error rather than silently succeeding twice, since the scheduler
// must never double-fire a completion notification for the same timer.
func (r *Repository) Complete(ctx context.Context, id int64, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE timers SET status = ?, ended_at = ?, paused_at = NULL
		WHERE id = ? AND status IN (?, ?)
	`, string(StatusCompleted), at.Unix(), id, string(StatusRunning), string(StatusPaused))
	if err != nil {
		return fmt.Errorf("timers: completing %d: %w", id, err)
	}
	return requireAffected(res, "timers: %d is already finished", id)
}

// Cancel marks a timer cancelled (section 21 - abandoning a timer
// without it counting as a completion). Same idempotency guard as
// Complete.
func (r *Repository) Cancel(ctx context.Context, id int64, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE timers SET status = ?, ended_at = ?, paused_at = NULL
		WHERE id = ? AND status IN (?, ?)
	`, string(StatusCancelled), at.Unix(), id, string(StatusRunning), string(StatusPaused))
	if err != nil {
		return fmt.Errorf("timers: cancelling %d: %w", id, err)
	}
	return requireAffected(res, "timers: %d is already finished", id)
}

// UpdateMetadata overwrites metadata_json - used only by the Pomodoro
// chain (internal/scheduler) to advance PomodoroState between phases,
// and kept separate from Create so non-Pomodoro callers never need to
// think about it.
func (r *Repository) UpdateMetadata(ctx context.Context, id int64, metadataJSON string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE timers SET metadata_json = ? WHERE id = ?`, metadataJSON, id)
	if err != nil {
		return fmt.Errorf("timers: updating metadata for %d: %w", id, err)
	}
	return nil
}

func requireAffected(res sql.Result, format string, id int64) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf(format, id)
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

func nullableDuration(secs int64) any {
	if secs <= 0 {
		return nil
	}
	return secs
}
