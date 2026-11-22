package sessions

import (
	"context"
	"database/sql"
	"fmt"
)

// View is a session joined with the human-readable names Timeline needs
// to display - application/project names and the project's color token
// - rather than making the frontend do a second round-trip per row to
// resolve IDs.
type View struct {
	ID              int64
	StartedAt       int64 // unix seconds
	EndedAt         int64
	ApplicationID   int64
	ApplicationName string
	ProjectID       int64 // 0 = unassigned
	ProjectName     string
	ProjectColor    string
	CategoryID      int64 // 0 = uncategorized
	CategoryName    string
	CategoryColor   string
	Confidence      float64
	ManuallyEdited  bool
	Excluded        bool
	Note            string
}

// Repository persists and queries sessions. Kept as an interface for the
// same testability reason as activity.Repository - the Activity Engine
// only calls InsertSession; the query/correction methods exist for
// app/services' Timeline-facing service.
type Repository interface {
	// InsertSession writes a closed session and its session_events
	// membership rows in a single transaction - a session and its event
	// links must never exist independently of each other.
	InsertSession(ctx context.Context, s BuiltSession) (int64, error)

	// ListForRange returns every session whose time range overlaps
	// [startUnix, endUnix), ordered chronologically - what the Timeline's
	// day/week/month views query directly.
	ListForRange(ctx context.Context, startUnix, endUnix int64) ([]View, error)

	// UpdateCorrection applies a manual correction (section 63):
	// reassigns the project and category, sets excluded/note, and marks
	// the session manually_edited so nothing automated overwrites the
	// person's own judgment later. projectID/categoryID of 0 clears that
	// assignment.
	UpdateCorrection(ctx context.Context, id int64, projectID, categoryID int64, excluded bool, note string) error

	// Split divides a session into two at splitAtUnix, which must fall
	// strictly between the session's start and end. Every activity_event
	// at or after splitAtUnix moves to the new (second) session. Both
	// resulting sessions are marked manually_edited. Returns the new
	// session's id.
	Split(ctx context.Context, id int64, splitAtUnix int64) (int64, error)

	// Merge combines two sessions into one, keyed on keepID (whose
	// application/project/note are preserved) - the [mergeID] session's
	// events are reassigned to keepID, keepID's time range is extended to
	// cover both, and the mergeID row is deleted. Marks the surviving
	// session manually_edited.
	Merge(ctx context.Context, keepID, mergeID int64) error

	// SumDurationByCategory returns total tracked seconds per category
	// (key 0 = uncategorized) for sessions overlapping the range,
	// excluding any session marked Excluded (section 63's "exclude from
	// analytics" applies to every aggregate, not just Timeline display).
	SumDurationByCategory(ctx context.Context, startUnix, endUnix int64) (map[int64]int64, error)

	// SumDurationByProject is SumDurationByCategory's project-scoped
	// twin (key 0 = unassigned).
	SumDurationByProject(ctx context.Context, startUnix, endUnix int64) (map[int64]int64, error)

	// ContextSwitchStats computes section 32's "digital friction"
	// numbers for a range: how many session-to-session transitions
	// occurred, the average and longest session length. Excluded
	// sessions are omitted from all three, same as the Sum* methods.
	ContextSwitchStats(ctx context.Context, startUnix, endUnix int64) (ContextSwitchStats, error)

	// LastActivityByProject returns, for every project that has ever
	// had at least one non-excluded session, the unix-second end time
	// of its single most recent one. This is the real signal "I HAVE
	// NO IDEA" (spec section 34, "neglected projects") uses to find a
	// genuinely under-touched project - not a guess, and not the same
	// thing SumDurationByProject answers (that's "how much time in a
	// window", this is "when was the last time at all").
	LastActivityByProject(ctx context.Context) (map[int64]int64, error)
}

// ContextSwitchStats is section 32's context-switching summary for a
// time range.
type ContextSwitchStats struct {
	Switches       int // count of session-to-session transitions (sessionCount - 1, or 0 if fewer than 2)
	SessionCount   int
	AverageSeconds int64
	LongestSeconds int64
}

type sqlRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &sqlRepository{db: db}
}

// nullableID converts the "0 means unset" convention BuiltSession.ProjectID
// uses into a genuine SQL NULL - projects.id is an AUTOINCREMENT primary
// key starting at 1, so 0 is never a real project id, but storing a
// literal 0 in a nullable foreign key column would be misleading (and
// would fail the FK constraint against a real row, since no project has
// id 0) where NULL correctly means "no project assigned yet."
func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

func (r *sqlRepository) InsertSession(ctx context.Context, s BuiltSession) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sessions: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	// project_id and category_id are both populated as of Phase 3/4 - see
	// BuiltSession's doc comments for how each gets resolved.
	res, err := tx.ExecContext(ctx, `
		INSERT INTO sessions (started_at, ended_at, application_id, project_id, category_id, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, unixepoch())
	`, s.StartedAt.Unix(), s.EndedAt.Unix(), s.ApplicationID, nullableID(s.ProjectID), nullableID(s.CategoryID), s.Confidence)
	if err != nil {
		return 0, fmt.Errorf("sessions: inserting session: %w", err)
	}

	sessionID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("sessions: reading new session id: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO session_events (session_id, event_id) VALUES (?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("sessions: preparing session_events insert: %w", err)
	}
	defer stmt.Close()

	for _, eventID := range s.EventIDs {
		if _, err := stmt.ExecContext(ctx, sessionID, eventID); err != nil {
			return 0, fmt.Errorf("sessions: linking event %d to session %d: %w", eventID, sessionID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sessions: committing: %w", err)
	}
	return sessionID, nil
}

func (r *sqlRepository) ListForRange(ctx context.Context, startUnix, endUnix int64) ([]View, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			s.id, s.started_at, s.ended_at,
			COALESCE(s.application_id, 0), COALESCE(a.process_name, ''),
			COALESCE(s.project_id, 0), COALESCE(p.name, ''), COALESCE(p.color_token, ''),
			COALESCE(s.category_id, 0), COALESCE(c.name, ''), COALESCE(c.color_token, ''),
			s.confidence, s.manually_edited, s.excluded, COALESCE(s.note, '')
		FROM sessions s
		LEFT JOIN applications a ON a.id = s.application_id
		LEFT JOIN projects p ON p.id = s.project_id
		LEFT JOIN categories c ON c.id = s.category_id
		WHERE s.started_at < ? AND s.ended_at > ?
		ORDER BY s.started_at ASC
	`, endUnix, startUnix)
	if err != nil {
		return nil, fmt.Errorf("sessions: listing range: %w", err)
	}
	defer rows.Close()

	var out []View
	for rows.Next() {
		var v View
		var manuallyEdited, excluded int
		if err := rows.Scan(
			&v.ID, &v.StartedAt, &v.EndedAt,
			&v.ApplicationID, &v.ApplicationName,
			&v.ProjectID, &v.ProjectName, &v.ProjectColor,
			&v.CategoryID, &v.CategoryName, &v.CategoryColor,
			&v.Confidence, &manuallyEdited, &excluded, &v.Note,
		); err != nil {
			return nil, fmt.Errorf("sessions: scanning row: %w", err)
		}
		v.ManuallyEdited = manuallyEdited != 0
		v.Excluded = excluded != 0
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *sqlRepository) UpdateCorrection(ctx context.Context, id int64, projectID, categoryID int64, excluded bool, note string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE sessions
		SET project_id = ?, category_id = ?, excluded = ?, note = ?, manually_edited = 1
		WHERE id = ?
	`, nullableID(projectID), nullableID(categoryID), boolToInt(excluded), note, id)
	if err != nil {
		return fmt.Errorf("sessions: updating correction for %d: %w", id, err)
	}
	return nil
}

func (r *sqlRepository) Split(ctx context.Context, id int64, splitAtUnix int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sessions: split: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var startedAt, endedAt, applicationID int64
	var projectID sql.NullInt64
	var confidence float64
	err = tx.QueryRowContext(ctx,
		`SELECT started_at, ended_at, application_id, project_id, confidence FROM sessions WHERE id = ?`, id,
	).Scan(&startedAt, &endedAt, &applicationID, &projectID, &confidence)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("sessions: no session with id %d", id)
	}
	if err != nil {
		return 0, fmt.Errorf("sessions: split: reading session %d: %w", id, err)
	}
	if splitAtUnix <= startedAt || splitAtUnix >= endedAt {
		return 0, fmt.Errorf("sessions: split point must fall strictly within the session's time range")
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE sessions SET ended_at = ?, manually_edited = 1 WHERE id = ?`, splitAtUnix, id,
	); err != nil {
		return 0, fmt.Errorf("sessions: split: shrinking original session: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO sessions (started_at, ended_at, application_id, project_id, category_id, confidence, manually_edited, created_at)
		VALUES (?, ?, ?, ?, NULL, ?, 1, unixepoch())
	`, splitAtUnix, endedAt, applicationID, projectID, confidence)
	if err != nil {
		return 0, fmt.Errorf("sessions: split: inserting new session: %w", err)
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("sessions: split: reading new session id: %w", err)
	}

	// Move every event at or after the split point to the new session.
	if _, err := tx.ExecContext(ctx, `
		UPDATE session_events
		SET session_id = ?
		WHERE session_id = ?
		  AND event_id IN (SELECT id FROM activity_events WHERE started_at >= ?)
	`, newID, id, splitAtUnix); err != nil {
		return 0, fmt.Errorf("sessions: split: reassigning events: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sessions: split: committing: %w", err)
	}
	return newID, nil
}

func (r *sqlRepository) Merge(ctx context.Context, keepID, mergeID int64) error {
	if keepID == mergeID {
		return fmt.Errorf("sessions: cannot merge a session with itself")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sessions: merge: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var keepStart, keepEnd, mergeStart, mergeEnd int64
	if err := tx.QueryRowContext(ctx, `SELECT started_at, ended_at FROM sessions WHERE id = ?`, keepID).Scan(&keepStart, &keepEnd); err != nil {
		return fmt.Errorf("sessions: merge: reading session %d: %w", keepID, err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT started_at, ended_at FROM sessions WHERE id = ?`, mergeID).Scan(&mergeStart, &mergeEnd); err != nil {
		return fmt.Errorf("sessions: merge: reading session %d: %w", mergeID, err)
	}

	newStart, newEnd := keepStart, keepEnd
	if mergeStart < newStart {
		newStart = mergeStart
	}
	if mergeEnd > newEnd {
		newEnd = mergeEnd
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE sessions SET started_at = ?, ended_at = ?, manually_edited = 1 WHERE id = ?`,
		newStart, newEnd, keepID,
	); err != nil {
		return fmt.Errorf("sessions: merge: extending kept session: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE session_events SET session_id = ? WHERE session_id = ?`, keepID, mergeID,
	); err != nil {
		return fmt.Errorf("sessions: merge: reassigning events: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, mergeID); err != nil {
		return fmt.Errorf("sessions: merge: deleting merged session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sessions: merge: committing: %w", err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r *sqlRepository) SumDurationByCategory(ctx context.Context, startUnix, endUnix int64) (map[int64]int64, error) {
	return r.sumDurationBy(ctx, "category_id", startUnix, endUnix)
}

func (r *sqlRepository) SumDurationByProject(ctx context.Context, startUnix, endUnix int64) (map[int64]int64, error) {
	return r.sumDurationBy(ctx, "project_id", startUnix, endUnix)
}

// sumDurationBy backs both Sum* methods - column is always one of the
// two fixed literals above (never user input), so string-building the
// column name into the query here does not reopen the parameterization
// requirement from section 53; every actual value (startUnix, endUnix)
// still goes through placeholders.
//
// Overlap seconds are clamped to the query range: a session that starts
// before startUnix or ends after endUnix only contributes the portion
// that actually falls inside [startUnix, endUnix), matching what
// Timeline already shows for the same range.
func (r *sqlRepository) sumDurationBy(ctx context.Context, column string, startUnix, endUnix int64) (map[int64]int64, error) {
	query := fmt.Sprintf(`
		SELECT COALESCE(%s, 0) AS key_id,
		       SUM(MIN(ended_at, ?) - MAX(started_at, ?)) AS total_seconds
		FROM sessions
		WHERE started_at < ? AND ended_at > ? AND excluded = 0
		GROUP BY key_id
	`, column)

	rows, err := r.db.QueryContext(ctx, query, endUnix, startUnix, endUnix, startUnix)
	if err != nil {
		return nil, fmt.Errorf("sessions: summing duration by %s: %w", column, err)
	}
	defer rows.Close()

	out := map[int64]int64{}
	for rows.Next() {
		var key, total int64
		if err := rows.Scan(&key, &total); err != nil {
			return nil, fmt.Errorf("sessions: scanning sum row: %w", err)
		}
		out[key] = total
	}
	return out, rows.Err()
}

func (r *sqlRepository) ContextSwitchStats(ctx context.Context, startUnix, endUnix int64) (ContextSwitchStats, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ended_at - started_at
		FROM sessions
		WHERE started_at < ? AND ended_at > ? AND excluded = 0
		ORDER BY started_at ASC
	`, endUnix, startUnix)
	if err != nil {
		return ContextSwitchStats{}, fmt.Errorf("sessions: context switch stats: %w", err)
	}
	defer rows.Close()

	var stats ContextSwitchStats
	var totalSeconds int64
	for rows.Next() {
		var duration int64
		if err := rows.Scan(&duration); err != nil {
			return ContextSwitchStats{}, fmt.Errorf("sessions: scanning duration row: %w", err)
		}
		stats.SessionCount++
		totalSeconds += duration
		if duration > stats.LongestSeconds {
			stats.LongestSeconds = duration
		}
	}
	if err := rows.Err(); err != nil {
		return ContextSwitchStats{}, err
	}

	if stats.SessionCount > 1 {
		stats.Switches = stats.SessionCount - 1
	}
	if stats.SessionCount > 0 {
		stats.AverageSeconds = totalSeconds / int64(stats.SessionCount)
	}
	return stats, nil
}

func (r *sqlRepository) LastActivityByProject(ctx context.Context) (map[int64]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT project_id, MAX(ended_at)
		FROM sessions
		WHERE excluded = 0 AND project_id IS NOT NULL
		GROUP BY project_id
	`)
	if err != nil {
		return nil, fmt.Errorf("sessions: last activity by project: %w", err)
	}
	defer rows.Close()

	out := map[int64]int64{}
	for rows.Next() {
		var projectID, lastEndedAt int64
		if err := rows.Scan(&projectID, &lastEndedAt); err != nil {
			return nil, fmt.Errorf("sessions: scanning last-activity row: %w", err)
		}
		out[projectID] = lastEndedAt
	}
	return out, rows.Err()
}
