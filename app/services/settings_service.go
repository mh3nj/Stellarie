// Package services holds STELLAREI's Wails v3 Services - the structs whose
// exported methods get bound to the frontend (application.NewService(&X{})
// in main.go). Per the framework's own convention, and the mandate in
// section 82 to avoid god objects, each service here should stay a thin
// adapter: validate input, call into a plain Go package for the actual
// logic, translate the result. Domain logic that doesn't need Wails lives
// outside this package so it stays testable as plain Go (see
// internal/closebehavior for the pattern).
package services

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"stellarei/internal/closebehavior"
	"stellarei/internal/privacy"
)

// Known setting keys. Centralised here so the frontend, the Settings UI,
// and every backend consumer agree on the exact string. Add to this list
// as later phases introduce new settings - don't invent ad-hoc keys inline.
const (
	KeyCloseBehavior    = closebehavior.SettingKey // "close" | "tray" | "ask"
	KeyStartupEnabled   = "startup_enabled"        // "true" | "false"
	KeyStartupBehavior  = "startup_behavior"       // "normal" | "tray" | "silent"
	KeyIdleThresholdSec = "idle_threshold_seconds" // integer, default 120
	KeyTheme            = "theme"                  // theme name, e.g. "Midnight"
	KeyLanguage         = "language"               // "en" | "fa" | "de"

	// Week-start override (manual setting, distinct from the automatic
	// locale default in frontend/src/lib/i18n.ts's getWeekStartDay).
	// Stored as ""/"auto" (follow the language) or "0".."6" (a JS
	// Date.getDay() index). First consumer is frontend/src/lib/week-start.ts
	// via the generic GetString/SetString above - no typed wrapper needed,
	// the same way KeyLanguage needed zero new Go code. Weekly budget/goal
	// windows (internal/periods) and search's "this week" (internal/search)
	// still use Monday-first pending backend threading; see README.
	KeyWeekStart = "week_start"

	// Privacy Center (Phase 9, spec sections 26-28) - defined in
	// internal/privacy (the same pattern KeyCloseBehavior already uses
	// for closebehavior.SettingKey) since internal/privacy's own
	// SQLSettingsProvider (used by the background Sweeper) needs the
	// identical string without importing this package and creating a
	// cycle.
	KeyPrivacyModeEnabled     = privacy.SettingKeyPrivacyModeEnabled
	KeyScreenshotEnabled      = privacy.SettingKeyScreenshotEnabled
	KeyScreenshotIntervalSecs = privacy.SettingKeyScreenshotIntervalSecs
	KeyScreenshotRetention    = privacy.SettingKeyScreenshotRetention
	KeyBrowserTrackingEnabled = privacy.SettingKeyBrowserTrackingEnabled

	// Global shortcuts (spec section 45, user-configurability half).
	// One key per action; an unset or empty value means "use the
	// built-in default" (internal/shortcuts.DefaultAccelerator) rather
	// than storing the default explicitly - so a future change to a
	// DEFAULT (not a user's own override) takes effect for anyone who
	// never customized it. Values are shortcuts.Action strings used
	// directly as key suffixes, not redeclared here, so the two can
	// never drift apart - see shortcuts_service.go.
	keyShortcutPrefix = "shortcut_"
)

// SettingsService is bound to the frontend via application.NewService in
// main.go. Every exported method here becomes a callable JS function once
// `wails3 generate bindings` runs.
type SettingsService struct {
	db *sql.DB
}

// NewSettingsService constructs a SettingsService. db must already be
// migrated (see internal/database.Open) - this service assumes the
// `settings` table exists.
func NewSettingsService(db *sql.DB) *SettingsService {
	return &SettingsService{db: db}
}

// GetString returns the raw string value for key, or "" if it has never
// been set. "Never set" is not distinguished from "set to empty string" at
// this layer - callers that care about that distinction should use a
// sentinel default and check for it, the same way any KV store works.
func (s *SettingsService) GetString(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("settings: get %q: %w", key, err)
	}
	return value, nil
}

// SetString upserts key to value. value_type is recorded as "string"
// use SetBool/SetInt for typed values so GetAll (added in a later phase,
// once the Settings UI needs to render the whole set at once) can render
// the right control for each key without guessing.
func (s *SettingsService) SetString(key, value string) error {
	return s.upsert(key, value, "string")
}

// GetBool parses the stored value as a bool. An unset or unparseable key
// returns false rather than erroring - every boolean setting in this app
// (screenshots, browser tracking, startup) is "off" until explicitly
// turned on, so false is always a safe default (section 58: privacy
// features ship off by default).
func (s *SettingsService) GetBool(key string) (bool, error) {
	raw, err := s.GetString(key)
	if err != nil {
		return false, err
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, nil
	}
	return b, nil
}

func (s *SettingsService) SetBool(key string, value bool) error {
	return s.upsert(key, strconv.FormatBool(value), "bool")
}

// GetInt parses the stored value as an int, returning fallback if the key
// is unset or unparseable. Wails-bound methods can't easily express
// "value or not found" as a single return without an extra bool, so
// callers pass their own default explicitly instead.
func (s *SettingsService) GetInt(key string, fallback int) (int, error) {
	raw, err := s.GetString(key)
	if err != nil {
		return fallback, err
	}
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback, nil
	}
	return n, nil
}

func (s *SettingsService) SetInt(key string, value int) error {
	return s.upsert(key, strconv.Itoa(value), "int")
}

// GetCloseBehavior and SetCloseBehavior are typed convenience wrappers
// around the generic string accessors above. They exist because
// close-behavior is read by main.go's window-close hook on every close
// attempt (a hot path relative to other settings) and because
// closebehavior.Valid gives us real validation instead of accepting any
// string blindly from the frontend.
func (s *SettingsService) GetCloseBehavior() (string, error) {
	raw, err := s.GetString(KeyCloseBehavior)
	if err != nil {
		return "", err
	}
	return string(closebehavior.Parse(raw)), nil
}

func (s *SettingsService) SetCloseBehavior(value string) error {
	if !closebehavior.Valid(value) {
		return fmt.Errorf("settings: invalid close behavior %q (want close, tray, or ask)", value)
	}
	return s.upsert(KeyCloseBehavior, value, "string")
}

func (s *SettingsService) upsert(key, value, valueType string) error {
	_, err := s.db.ExecContext(context.Background(), `
		INSERT INTO settings (key, value, value_type, updated_at)
		VALUES (?, ?, ?, unixepoch())
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			value_type = excluded.value_type,
			updated_at = excluded.updated_at
	`, key, value, valueType)
	if err != nil {
		return fmt.Errorf("settings: set %q: %w", key, err)
	}
	return nil
}
