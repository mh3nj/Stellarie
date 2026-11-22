package privacy

import (
	"context"
	"database/sql"
	"strconv"
)

// Setting key constants for everything this package's UI controls
// (spec section 54's "Tracking"/"Privacy" settings sections) - defined
// here, the same way internal/closebehavior.SettingKey is, so
// app/services.SettingsService (which the frontend actually calls for
// these - see that file's own constant block) references the identical
// string rather than a second hand-typed copy that could drift.
const (
	SettingKeyPrivacyModeEnabled     = "privacy_mode_enabled"
	SettingKeyScreenshotEnabled      = "screenshot_enabled"       // "true" | "false", default false (section 27: OFF by default)
	SettingKeyScreenshotIntervalSecs = "screenshot_interval_secs" // integer seconds, e.g. 300 for 5 minutes
	SettingKeyScreenshotRetention    = "screenshot_retention_days"
	SettingKeyBrowserTrackingEnabled = "browser_tracking_enabled" // "true" | "false", default false (section 28: OFF by default)
)

// SQLSettingsProvider implements SettingsProvider by reading the
// `settings` key-value table directly - the same table
// app/services.SettingsService reads/writes, so a change the person
// makes through the Settings UI takes effect on the Sweeper's very next
// tick with no extra plumbing.
type SQLSettingsProvider struct {
	db *sql.DB
}

func NewSQLSettingsProvider(db *sql.DB) *SQLSettingsProvider {
	return &SQLSettingsProvider{db: db}
}

func (p *SQLSettingsProvider) ScreenshotSettings(ctx context.Context) (enabled bool, intervalSecs int, retention RetentionDays, err error) {
	enabled, err = p.getBool(ctx, SettingKeyScreenshotEnabled)
	if err != nil {
		return false, 0, 0, err
	}
	intervalSecs, err = p.getInt(ctx, SettingKeyScreenshotIntervalSecs, 300) // 5-minute default, matching section 27's own middle option
	if err != nil {
		return false, 0, 0, err
	}
	retentionRaw, err := p.getInt(ctx, SettingKeyScreenshotRetention, int(RetentionSevenDays))
	if err != nil {
		return false, 0, 0, err
	}
	return enabled, intervalSecs, RetentionDays(retentionRaw), nil
}

func (p *SQLSettingsProvider) getBool(ctx context.Context, key string) (bool, error) {
	var value string
	err := p.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return value == "true", nil
}

func (p *SQLSettingsProvider) getInt(ctx context.Context, key string, fallback int) (int, error) {
	var value string
	err := p.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return fallback, nil
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback, nil // a corrupted/hand-edited setting value falls back rather than breaking the sweep
	}
	return n, nil
}
