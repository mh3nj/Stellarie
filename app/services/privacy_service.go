package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"stellarei/internal/activity"
	"stellarei/internal/privacy"
)

// ExclusionView/ApplicationPrivacyView/ScreenshotView are privacy.*
// shaped for the frontend - unix-second timestamps and plain strings
// instead of the domain package's typed enums.
type ExclusionView struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"` // application | window_title_pattern | domain
	Pattern   string `json:"pattern"`
	AppliesTo string `json:"appliesTo"` // all | screenshots | browser_tracking
	CreatedAt int64  `json:"createdAt"`
}

type ApplicationPrivacyView struct {
	ApplicationID      int64  `json:"applicationId"`
	ProcessName        string `json:"processName"`
	TrackingEnabled    bool   `json:"trackingEnabled"`
	ScreenshotAllowed  bool   `json:"screenshotAllowed"`
	BrowserURLTracking bool   `json:"browserUrlTracking"`
	PrivacySensitive   bool   `json:"privacySensitive"`
}

type ScreenshotView struct {
	ID            int64  `json:"id"`
	TakenAt       int64  `json:"takenAt"`
	FilePath      string `json:"filePath"`
	ApplicationID int64  `json:"applicationId,omitempty"`
	DeleteAfter   int64  `json:"deleteAfter,omitempty"`
}

// PrivacyService is bound to the frontend via application.NewService in
// main.go. Privacy Mode's on/off (which also has to reach into the
// running Activity Engine, not just the database) lives here;
// screenshot ON/OFF, interval, and retention are plain settings the
// frontend reads/writes directly via SettingsService using the
// KeyScreenshot* constants - no need to duplicate simple key-value
// plumbing behind a second service (the same reasoning
// KeyIdleThresholdSec already follows for ActivityService).
type PrivacyService struct {
	repo   *privacy.Repository
	engine *activity.Engine
	db     *sql.DB
}

func NewPrivacyService(repo *privacy.Repository, engine *activity.Engine, db *sql.DB) *PrivacyService {
	return &PrivacyService{repo: repo, engine: engine, db: db}
}

// GetPrivacyMode returns whether Privacy Mode is currently active
// read from the live Engine (the actual source of truth for whether
// tracking is stopped right now), not just the persisted setting, so a
// stale setting value can never disagree with reality.
func (s *PrivacyService) GetPrivacyMode() bool {
	return s.engine.Status().PrivacyMode
}

// SetPrivacyMode turns Privacy Mode on or off (spec section 26):
// updates the live Engine immediately AND persists the setting, so the
// state survives an app restart (spec section 26's own "PRIVACY MODE
// ACTIVE" banner should still be showing if the person reopens
// STELLAREI while it was on).
func (s *PrivacyService) SetPrivacyMode(enabled bool) error {
	s.engine.SetPrivacyMode(enabled)
	value := "false"
	if enabled {
		value = "true"
	}
	_, err := s.db.ExecContext(context.Background(), `
		INSERT INTO settings (key, value, value_type, updated_at) VALUES (?, ?, 'bool', unixepoch())
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, value_type = 'bool', updated_at = excluded.updated_at
	`, KeyPrivacyModeEnabled, value)
	if err != nil {
		return fmt.Errorf("privacy: persisting privacy mode setting: %w", err)
	}
	return nil
}

// --- exclusions ----------------------------------------------------------

func (s *PrivacyService) ListExclusions() ([]ExclusionView, error) {
	rows, err := s.repo.ListExclusions(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]ExclusionView, 0, len(rows))
	for _, e := range rows {
		out = append(out, ExclusionView{ID: e.ID, Type: string(e.Type), Pattern: e.Pattern, AppliesTo: string(e.AppliesTo), CreatedAt: e.CreatedAt.Unix()})
	}
	return out, nil
}

func (s *PrivacyService) CreateExclusion(exclusionType, pattern, appliesTo string) (int64, error) {
	return s.repo.CreateExclusion(context.Background(), privacy.ExclusionType(exclusionType), pattern, privacy.AppliesTo(appliesTo))
}

func (s *PrivacyService) DeleteExclusion(id int64) error {
	return s.repo.DeleteExclusion(context.Background(), id)
}

// --- application privacy --------------------------------------------------

func (s *PrivacyService) ListApplicationPrivacy() ([]ApplicationPrivacyView, error) {
	rows, err := s.repo.ListApplicationPrivacy(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]ApplicationPrivacyView, 0, len(rows))
	for _, a := range rows {
		out = append(out, ApplicationPrivacyView{
			ApplicationID: a.ApplicationID, ProcessName: a.ProcessName,
			TrackingEnabled: a.TrackingEnabled, ScreenshotAllowed: a.ScreenshotAllowed,
			BrowserURLTracking: a.BrowserURLTracking, PrivacySensitive: a.PrivacySensitive,
		})
	}
	return out, nil
}

func (s *PrivacyService) SetApplicationPrivacy(applicationID int64, trackingEnabled, screenshotAllowed, browserURLTracking, privacySensitive bool) error {
	return s.repo.SetApplicationPrivacy(context.Background(), applicationID, trackingEnabled, screenshotAllowed, browserURLTracking, privacySensitive)
}

// --- screenshots -----------------------------------------------------------

func (s *PrivacyService) ListScreenshots(limit int) ([]ScreenshotView, error) {
	rows, err := s.repo.ListScreenshots(context.Background(), limit)
	if err != nil {
		return nil, err
	}
	out := make([]ScreenshotView, 0, len(rows))
	for _, sc := range rows {
		v := ScreenshotView{ID: sc.ID, TakenAt: sc.TakenAt.Unix(), FilePath: sc.FilePath, ApplicationID: sc.ApplicationID}
		if sc.DeleteAfter != nil {
			v.DeleteAfter = sc.DeleteAfter.Unix()
		}
		out = append(out, v)
	}
	return out, nil
}

// DeleteScreenshot removes both the database record and the underlying
// file - a person deleting a screenshot from the UI expects it gone,
// not just hidden from the list. Looks the path up itself (one extra
// query) rather than asking the frontend to pass it back, so a stale or
// tampered client-side path can never be used for the actual file
// removal.
func (s *PrivacyService) DeleteScreenshot(id int64) error {
	ctx := context.Background()
	var filePath string
	err := s.db.QueryRowContext(ctx, `SELECT file_path FROM screenshots WHERE id = ?`, id).Scan(&filePath)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("privacy: looking up screenshot %d: %w", id, err)
	}
	if filePath != "" {
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			// Section 56: a stuck file (permissions, already open
			// elsewhere) must not prevent clearing the history entry
			// the person asked to delete it; losing the DB row is worse
			// than a leftover file they can clean up manually.
			log.Printf("privacy: removing screenshot file %s: %v", filePath, err)
		}
	}
	return s.repo.DeleteScreenshot(ctx, id)
}
