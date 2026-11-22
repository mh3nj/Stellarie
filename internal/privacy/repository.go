package privacy

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository is the SQL-backed store for everything this package owns:
// privacy_exclusions, screenshots, and the privacy-relevant columns of
// applications (tracking_enabled, screenshot_allowed,
// browser_url_tracking, privacy_sensitive) - see
// internal/categories.Repository for the category-assignment columns of
// that same table, owned separately.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// --- exclusions --------------------------------------------------------

func (r *Repository) ListExclusions(ctx context.Context) ([]Exclusion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, exclusion_type, pattern, applies_to, created_at
		FROM privacy_exclusions ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("privacy: listing exclusions: %w", err)
	}
	defer rows.Close()

	var out []Exclusion
	for rows.Next() {
		var e Exclusion
		var exclusionType, appliesTo string
		var createdAtUnix int64
		if err := rows.Scan(&e.ID, &exclusionType, &e.Pattern, &appliesTo, &createdAtUnix); err != nil {
			return nil, fmt.Errorf("privacy: scanning exclusion row: %w", err)
		}
		e.Type = ExclusionType(exclusionType)
		e.AppliesTo = AppliesTo(appliesTo)
		e.CreatedAt = time.Unix(createdAtUnix, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) CreateExclusion(ctx context.Context, exclusionType ExclusionType, pattern string, appliesTo AppliesTo) (int64, error) {
	if pattern == "" {
		return 0, fmt.Errorf("privacy: exclusion pattern must not be empty")
	}
	if appliesTo == "" {
		appliesTo = AppliesToAll
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO privacy_exclusions (exclusion_type, pattern, applies_to, created_at)
		VALUES (?, ?, ?, unixepoch())
	`, string(exclusionType), pattern, string(appliesTo))
	if err != nil {
		return 0, fmt.Errorf("privacy: creating exclusion: %w", err)
	}
	return res.LastInsertId()
}

func (r *Repository) DeleteExclusion(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM privacy_exclusions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("privacy: deleting exclusion %d: %w", id, err)
	}
	return nil
}

// --- application privacy ------------------------------------------------

func (r *Repository) ListApplicationPrivacy(ctx context.Context) ([]ApplicationPrivacy, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, process_name, tracking_enabled, screenshot_allowed, browser_url_tracking, privacy_sensitive
		FROM applications ORDER BY process_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("privacy: listing application privacy settings: %w", err)
	}
	defer rows.Close()

	var out []ApplicationPrivacy
	for rows.Next() {
		var a ApplicationPrivacy
		var tracking, screenshot, browser, sensitive int
		if err := rows.Scan(&a.ApplicationID, &a.ProcessName, &tracking, &screenshot, &browser, &sensitive); err != nil {
			return nil, fmt.Errorf("privacy: scanning application row: %w", err)
		}
		a.TrackingEnabled, a.ScreenshotAllowed = tracking != 0, screenshot != 0
		a.BrowserURLTracking, a.PrivacySensitive = browser != 0, sensitive != 0
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetApplicationPrivacy resolves a Context's privacy-relevant flags for
// one application, defaulting to the safe/permissive built-in values
// (tracking on, screenshots on, browser tracking off, not sensitive
// matching the applications table's own DEFAULTs) for an application id
// that isn't in the registry yet, since a session referencing an
// application must never crash a privacy check.
func (r *Repository) GetApplicationPrivacy(ctx context.Context, applicationID int64) (ApplicationPrivacy, error) {
	if applicationID == 0 {
		return ApplicationPrivacy{TrackingEnabled: true, ScreenshotAllowed: true}, nil
	}
	var a ApplicationPrivacy
	var tracking, screenshot, browser, sensitive int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, process_name, tracking_enabled, screenshot_allowed, browser_url_tracking, privacy_sensitive
		FROM applications WHERE id = ?
	`, applicationID).Scan(&a.ApplicationID, &a.ProcessName, &tracking, &screenshot, &browser, &sensitive)
	if err == sql.ErrNoRows {
		return ApplicationPrivacy{ApplicationID: applicationID, TrackingEnabled: true, ScreenshotAllowed: true}, nil
	}
	if err != nil {
		return ApplicationPrivacy{}, fmt.Errorf("privacy: getting application %d: %w", applicationID, err)
	}
	a.TrackingEnabled, a.ScreenshotAllowed = tracking != 0, screenshot != 0
	a.BrowserURLTracking, a.PrivacySensitive = browser != 0, sensitive != 0
	return a, nil
}

func (r *Repository) SetApplicationPrivacy(ctx context.Context, applicationID int64, trackingEnabled, screenshotAllowed, browserURLTracking, privacySensitive bool) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE applications
		SET tracking_enabled = ?, screenshot_allowed = ?, browser_url_tracking = ?, privacy_sensitive = ?
		WHERE id = ?
	`, boolToInt(trackingEnabled), boolToInt(screenshotAllowed), boolToInt(browserURLTracking), boolToInt(privacySensitive), applicationID)
	if err != nil {
		return fmt.Errorf("privacy: setting privacy for application %d: %w", applicationID, err)
	}
	return nil
}

// --- screenshots ---------------------------------------------------------

func (r *Repository) InsertScreenshot(ctx context.Context, takenAt time.Time, filePath string, sessionID, applicationID int64, deleteAfter *time.Time) (int64, error) {
	var deleteAfterArg any
	if deleteAfter != nil {
		deleteAfterArg = deleteAfter.Unix()
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO screenshots (taken_at, file_path, session_id, application_id, delete_after)
		VALUES (?, ?, ?, ?, ?)
	`, takenAt.Unix(), filePath, nullableID(sessionID), nullableID(applicationID), deleteAfterArg)
	if err != nil {
		return 0, fmt.Errorf("privacy: inserting screenshot: %w", err)
	}
	return res.LastInsertId()
}

func (r *Repository) ListScreenshots(ctx context.Context, limit int) ([]Screenshot, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, taken_at, file_path, COALESCE(session_id,0), COALESCE(application_id,0), delete_after
		FROM screenshots ORDER BY taken_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("privacy: listing screenshots: %w", err)
	}
	defer rows.Close()

	var out []Screenshot
	for rows.Next() {
		var s Screenshot
		var takenAtUnix int64
		var deleteAfterUnix sql.NullInt64
		if err := rows.Scan(&s.ID, &takenAtUnix, &s.FilePath, &s.SessionID, &s.ApplicationID, &deleteAfterUnix); err != nil {
			return nil, fmt.Errorf("privacy: scanning screenshot row: %w", err)
		}
		s.TakenAt = time.Unix(takenAtUnix, 0)
		if deleteAfterUnix.Valid {
			when := time.Unix(deleteAfterUnix.Int64, 0)
			s.DeleteAfter = &when
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListExpiredScreenshots returns every screenshot whose delete_after has
// arrived - the sweep's own query rather than fetching everything and
// filtering with ShouldPurge in Go, since a long-running install could
// have far more screenshot rows than are useful to pull over the wire
// just to check a timestamp.
func (r *Repository) ListExpiredScreenshots(ctx context.Context, now time.Time) ([]Screenshot, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, taken_at, file_path, COALESCE(session_id,0), COALESCE(application_id,0), delete_after
		FROM screenshots WHERE delete_after IS NOT NULL AND delete_after <= ?
	`, now.Unix())
	if err != nil {
		return nil, fmt.Errorf("privacy: listing expired screenshots: %w", err)
	}
	defer rows.Close()

	var out []Screenshot
	for rows.Next() {
		var s Screenshot
		var takenAtUnix int64
		var deleteAfterUnix sql.NullInt64
		if err := rows.Scan(&s.ID, &takenAtUnix, &s.FilePath, &s.SessionID, &s.ApplicationID, &deleteAfterUnix); err != nil {
			return nil, fmt.Errorf("privacy: scanning expired screenshot row: %w", err)
		}
		s.TakenAt = time.Unix(takenAtUnix, 0)
		if deleteAfterUnix.Valid {
			when := time.Unix(deleteAfterUnix.Int64, 0)
			s.DeleteAfter = &when
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) DeleteScreenshot(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM screenshots WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("privacy: deleting screenshot %d: %w", id, err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}
