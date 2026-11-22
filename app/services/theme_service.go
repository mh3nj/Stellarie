package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"stellarei/internal/theme"
)

// defaultActiveTheme is used when settings.theme has never been set
// first run, or a database that predates this service.
const defaultActiveTheme = "Midnight"

// ThemeRecord is what ListThemes returns to the frontend. TokensJSON is
// passed through as a raw JSON string rather than a typed map - the
// frontend's theme token shape (frontend/src/themes/themes.ts) is the
// source of truth for what a token set looks like, and re-declaring that
// shape in Go would just be one more place for the two to drift apart.
type ThemeRecord struct {
	Name       string `json:"name"`
	IsBuiltin  bool   `json:"isBuiltin"`
	TokensJSON string `json:"tokensJson"`
}

// ThemeService is bound to the frontend for Theme Studio (theme list,
// save/delete custom themes, switch the active theme) and also drives the
// tray icon's light/dark switching from the Go side via
// SetOnActiveThemeChange - see main.go.
type ThemeService struct {
	db       *sql.DB
	settings *SettingsService

	onActiveThemeChange func(mode theme.Mode, tokensJSON string)
}

func NewThemeService(db *sql.DB, settings *SettingsService) *ThemeService {
	return &ThemeService{db: db, settings: settings}
}

// SetOnActiveThemeChange installs the callback main.go uses to update the
// tray icon (and anything else that needs to react) whenever the active
// theme changes - via SetActiveThemeName, or via SaveTheme editing the
// theme that's currently active. Same Bind-after-construct rationale as
// WindowService.Bind: main.go builds the tray (which needs this service
// to exist first) before it has anything to wire this callback to.
func (t *ThemeService) SetOnActiveThemeChange(fn func(mode theme.Mode, tokensJSON string)) {
	t.onActiveThemeChange = fn
}

// ListThemes returns every theme, built-in and custom, ordered with
// built-ins first (so Theme Studio's list doesn't reshuffle every time a
// custom theme is added), then alphabetically.
func (t *ThemeService) ListThemes() ([]ThemeRecord, error) {
	rows, err := t.db.QueryContext(context.Background(), `
		SELECT name, is_builtin, tokens_json
		FROM themes
		ORDER BY is_builtin DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("theme: listing themes: %w", err)
	}
	defer rows.Close()

	// Initialized empty, never nil - see receipt_service.go's comment
	// on why a view-carrying slice must never marshal as null.
	out := make([]ThemeRecord, 0)
	for rows.Next() {
		var rec ThemeRecord
		var isBuiltin int
		if err := rows.Scan(&rec.Name, &isBuiltin, &rec.TokensJSON); err != nil {
			return nil, fmt.Errorf("theme: scanning theme row: %w", err)
		}
		rec.IsBuiltin = isBuiltin != 0
		out = append(out, rec)
	}
	return out, rows.Err()
}

// GetTokensJSON returns one theme's raw tokens JSON by name.
func (t *ThemeService) GetTokensJSON(name string) (string, error) {
	var tokensJSON string
	err := t.db.QueryRowContext(context.Background(),
		`SELECT tokens_json FROM themes WHERE name = ?`, name,
	).Scan(&tokensJSON)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("theme: no theme named %q", name)
	}
	if err != nil {
		return "", fmt.Errorf("theme: reading %q: %w", name, err)
	}
	return tokensJSON, nil
}

// SaveTheme creates or updates a CUSTOM theme (is_builtin is always 0 for
// anything written through this method - built-ins are only ever seeded
// by migrations/0003_seed_builtin_themes.sql). Refuses to silently
// overwrite a built-in theme under the same name, and does minimal
// server-side validation (valid JSON, non-empty) - the frontend's
// validateTheme does the full shape check before ever calling this, per
// section 40's "reject malformed files safely."
func (t *ThemeService) SaveTheme(name string, tokensJSON string) error {
	if name == "" {
		return fmt.Errorf("theme: name must not be empty")
	}
	if !json.Valid([]byte(tokensJSON)) {
		return fmt.Errorf("theme: tokens are not valid JSON")
	}

	ctx := context.Background()

	var existingIsBuiltin int
	err := t.db.QueryRowContext(ctx, `SELECT is_builtin FROM themes WHERE name = ?`, name).Scan(&existingIsBuiltin)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("theme: checking existing theme %q: %w", name, err)
	}
	if err == nil && existingIsBuiltin != 0 {
		return fmt.Errorf("theme: %q is a built-in theme and cannot be overwritten - save under a different name", name)
	}

	_, err = t.db.ExecContext(ctx, `
		INSERT INTO themes (name, is_builtin, tokens_json, created_at, updated_at)
		VALUES (?, 0, ?, unixepoch(), unixepoch())
		ON CONFLICT(name) DO UPDATE SET
			tokens_json = excluded.tokens_json,
			updated_at = excluded.updated_at
	`, name, tokensJSON)
	if err != nil {
		return fmt.Errorf("theme: saving %q: %w", name, err)
	}

	// If the theme being saved is also the currently active one, the
	// edit takes effect immediately - otherwise a user editing their
	// active theme in Theme Studio would see no visible change until
	// they separately re-activated it, which reads as broken, not
	// intentional.
	activeName, err := t.GetActiveThemeName()
	if err == nil && activeName == name {
		t.notifyActiveThemeChanged(tokensJSON)
	}

	return nil
}

// DeleteTheme removes a custom theme. Refuses to delete a built-in theme
// or the currently active theme (deleting your own active theme out from
// under yourself would leave nothing applied).
func (t *ThemeService) DeleteTheme(name string) error {
	ctx := context.Background()

	var isBuiltin int
	err := t.db.QueryRowContext(ctx, `SELECT is_builtin FROM themes WHERE name = ?`, name).Scan(&isBuiltin)
	if err == sql.ErrNoRows {
		return fmt.Errorf("theme: no theme named %q", name)
	}
	if err != nil {
		return fmt.Errorf("theme: checking %q: %w", name, err)
	}
	if isBuiltin != 0 {
		return fmt.Errorf("theme: %q is a built-in theme and cannot be deleted", name)
	}

	activeName, err := t.GetActiveThemeName()
	if err == nil && activeName == name {
		return fmt.Errorf("theme: %q is currently active - switch to another theme before deleting it", name)
	}

	if _, err := t.db.ExecContext(ctx, `DELETE FROM themes WHERE name = ?`, name); err != nil {
		return fmt.Errorf("theme: deleting %q: %w", name, err)
	}
	return nil
}

// GetActiveThemeName returns the name of the currently active theme,
// defaulting to "Midnight" if never set.
func (t *ThemeService) GetActiveThemeName() (string, error) {
	name, err := t.settings.GetString(KeyTheme)
	if err != nil {
		return "", err
	}
	if name == "" {
		return defaultActiveTheme, nil
	}
	return name, nil
}

// SetActiveThemeName switches the active theme and notifies the
// OnActiveThemeChange callback with its computed mode - this is what
// drives the tray icon switch (and anything else in the app that reacts
// to theme mode) from the Go side.
func (t *ThemeService) SetActiveThemeName(name string) error {
	tokensJSON, err := t.GetTokensJSON(name)
	if err != nil {
		return err
	}
	if err := t.settings.SetString(KeyTheme, name); err != nil {
		return fmt.Errorf("theme: persisting active theme: %w", err)
	}
	t.notifyActiveThemeChanged(tokensJSON)
	return nil
}

func (t *ThemeService) notifyActiveThemeChanged(tokensJSON string) {
	if t.onActiveThemeChange == nil {
		return
	}
	var parsed struct {
		Background string `json:"background"`
	}
	if err := json.Unmarshal([]byte(tokensJSON), &parsed); err != nil || parsed.Background == "" {
		return // malformed tokens shouldn't crash the notification path - the tray just keeps its last icon
	}
	mode, err := theme.ComputeMode(parsed.Background)
	if err != nil {
		return
	}
	t.onActiveThemeChange(mode, tokensJSON)
}
