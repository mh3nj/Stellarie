package ai

import (
	"context"
	"database/sql"
	"strconv"
)

// Setting key constants for Settings > AI (spec section 71's "AI:
// provider, local endpoint, model, permissions, data scope"). Defined
// here, the same way internal/privacy.SettingKey* are, so
// app/services.AIService (the thin Wails-bound adapter) references the
// identical strings rather than a second hand-typed copy that could
// drift - see internal/privacy/settings.go for the precedent this
// follows.
const (
	SettingKeyEnabled  = "ai_enabled"  // "true" | "false", default false (section 37: disabled by default)
	SettingKeyProvider = "ai_provider" // one of the Provider constants above
	SettingKeyEndpoint = "ai_endpoint" // e.g. "http://localhost:11434"
	SettingKeyModel    = "ai_model"

	// scopeKeyPrefix + Scope gives one settings key per feature checkbox,
	// e.g. "ai_scope_daily_reviews" - so enabling/disabling a single
	// scope is one row update, not a rewrite of some combined bitmask
	// value that would be opaque to inspect directly in the database.
	scopeKeyPrefix = "ai_scope_"
)

// ScopeSettingKey returns the settings-table key for one scope's
// individual opt-in checkbox.
func ScopeSettingKey(scope Scope) string {
	return scopeKeyPrefix + string(scope)
}

// SQLSettingsStore reads and writes AI Config from/to the same
// `settings` key-value table app/services.SettingsService already owns
// (see that file's doc comment on the shared table), so a change made
// through the Settings UI takes effect on the very next Enhance call
// with no extra plumbing - identical in spirit to
// internal/privacy.SQLSettingsProvider.
type SQLSettingsStore struct {
	db *sql.DB
}

func NewSQLSettingsStore(db *sql.DB) *SQLSettingsStore {
	return &SQLSettingsStore{db: db}
}

// Load reads the currently saved Config. A fresh install (no rows yet)
// loads as the safe all-off default: Enabled=false, Provider=ProviderNone,
// no scopes - never as "on" by omission.
func (s *SQLSettingsStore) Load(ctx context.Context) (Config, error) {
	enabled, err := s.getBool(ctx, SettingKeyEnabled)
	if err != nil {
		return Config{}, err
	}
	providerRaw, err := s.getString(ctx, SettingKeyProvider)
	if err != nil {
		return Config{}, err
	}
	endpoint, err := s.getString(ctx, SettingKeyEndpoint)
	if err != nil {
		return Config{}, err
	}
	model, err := s.getString(ctx, SettingKeyModel)
	if err != nil {
		return Config{}, err
	}

	provider := Provider(providerRaw)
	if providerRaw == "" || !ValidProvider(providerRaw) {
		provider = ProviderNone // an unset or corrupted value never accidentally activates a provider
	}

	scopes := make(map[Scope]bool, len(AllScopes))
	for _, sc := range AllScopes {
		on, err := s.getBool(ctx, ScopeSettingKey(sc))
		if err != nil {
			return Config{}, err
		}
		scopes[sc] = on
	}

	return Config{
		Enabled:  enabled,
		Provider: provider,
		Endpoint: endpoint,
		Model:    model,
		Scopes:   scopes,
	}, nil
}

// SetEnabled, SetProvider, SetEndpoint, SetModel, and SetScope are
// individually addressable (rather than one big SaveConfig(Config)) so
// the Settings UI can persist each field change immediately as the
// person edits it, matching how every other Settings section in this
// codebase already behaves (see app/services.SettingsService).

func (s *SQLSettingsStore) SetEnabled(ctx context.Context, enabled bool) error {
	return s.setBool(ctx, SettingKeyEnabled, enabled)
}

func (s *SQLSettingsStore) SetProvider(ctx context.Context, provider Provider) error {
	return s.setString(ctx, SettingKeyProvider, string(provider))
}

func (s *SQLSettingsStore) SetEndpoint(ctx context.Context, endpoint string) error {
	return s.setString(ctx, SettingKeyEndpoint, endpoint)
}

func (s *SQLSettingsStore) SetModel(ctx context.Context, model string) error {
	return s.setString(ctx, SettingKeyModel, model)
}

func (s *SQLSettingsStore) SetScope(ctx context.Context, scope Scope, on bool) error {
	return s.setBool(ctx, ScopeSettingKey(scope), on)
}

func (s *SQLSettingsStore) getString(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *SQLSettingsStore) getBool(ctx context.Context, key string) (bool, error) {
	raw, err := s.getString(ctx, key)
	if err != nil {
		return false, err
	}
	return raw == "true", nil
}

func (s *SQLSettingsStore) setString(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, value_type, updated_at)
		VALUES (?, ?, 'string', unixepoch())
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			value_type = excluded.value_type,
			updated_at = excluded.updated_at
	`, key, value)
	return err
}

func (s *SQLSettingsStore) setBool(ctx context.Context, key string, value bool) error {
	return s.setString(ctx, key, strconv.FormatBool(value))
}
