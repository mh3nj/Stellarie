package services

import (
	"context"
	"database/sql"
	"fmt"

	"stellarei/internal/ai"
)

// Known AI setting keys, re-exported the same way KeyPrivacyModeEnabled
// etc. re-export internal/privacy's constants above - so anything
// reading raw settings (Diagnostics, a future export) has one name to
// import rather than reaching into internal/ai directly.
const (
	KeyAIEnabled  = ai.SettingKeyEnabled
	KeyAIProvider = ai.SettingKeyProvider
	KeyAIEndpoint = ai.SettingKeyEndpoint
	KeyAIModel    = ai.SettingKeyModel
)

// AIConfigView is internal/ai.Config shaped for the frontend: plain
// strings and a string-keyed map instead of the domain package's typed
// Provider/Scope, the same translation PrivacyService's *View types do
// for internal/privacy.
type AIConfigView struct {
	Enabled  bool            `json:"enabled"`
	Provider string          `json:"provider"` // "none" | "ollama" | "openai_compatible"
	Endpoint string          `json:"endpoint"`
	Model    string          `json:"model"`
	Scopes   map[string]bool `json:"scopes"` // keyed by Scope string, e.g. "daily_reviews"
}

// AIResultView is ai.Result shaped for the frontend. LatencyMs is 0 when
// no call was actually attempted (e.g. AI is off).
type AIResultView struct {
	Text        string `json:"text"`
	Unavailable bool   `json:"unavailable"`
	Reason      string `json:"reason"`
	LatencyMs   int64  `json:"latencyMs"`
}

// AIDiagnosticsView is ai.LastCallInfo shaped for the frontend's
// Diagnostics page (spec section 55). HasRun distinguishes "nothing has
// called AI yet this run" (every field else zero) from a genuine past
// failure with an empty-looking reason.
type AIDiagnosticsView struct {
	HasRun    bool   `json:"hasRun"`
	At        string `json:"at"` // RFC 3339, "" if !HasRun
	Scope     string `json:"scope"`
	Provider  string `json:"provider"`
	Success   bool   `json:"success"`
	Reason    string `json:"reason"`
	LatencyMs int64  `json:"latencyMs"`
}

// AIService is bound to the frontend via application.NewService in
// main.go. It owns Settings > AI (section 71: provider, local endpoint,
// model, permissions/data scope) and the "Test Connection" action.
//
// Every setter here persists through store immediately and then applies
// the freshly-loaded Config to svc via SetConfig - so "the change I just
// made in Settings" and "what the next AI-backed action will actually
// use" can never drift apart, and a restored backup or a hand-edited
// settings row takes effect on the very next call without needing a
// restart (unlike, say, a database swap - see BackupService's restart-to-
// apply doc comment for the contrasting case where that ISN'T possible).
//
// Actually calling out to AI to enhance a specific feature (Daily
// Review commentary, Pattern Discovery phrasing, "I HAVE NO IDEA")
// belongs in each of those services, which should hold their own
// reference to Service and call Enhance with the appropriate Scope
// this file only owns configuration and the standalone connection test,
// consistent with section 82's one-responsibility-per-service guidance.
type AIService struct {
	db    *sql.DB
	store *ai.SQLSettingsStore
	svc   *ai.Service
}

// NewAIService constructs an AIService and immediately loads whatever
// configuration was already saved (e.g. from a previous run, or a
// restored backup) into the underlying ai.Service - so a person who
// configured AI yesterday doesn't find every feature silently back to
// "off" after restarting the app today. If that initial load fails for
// any reason, the underlying service is left at the zero Config (AI
// off) rather than main.go's startup sequence failing over it - a
// Settings-loading hiccup falling back to "AI just stays off until you
// revisit Settings" is exactly the graceful-degradation shape section 56
// asks for everywhere else, applied here too.
func NewAIService(db *sql.DB) *AIService {
	a := &AIService{
		db:    db,
		store: ai.NewSQLSettingsStore(db),
		svc:   ai.NewService(ai.Config{}),
	}
	_ = a.loadAndApply() // best-effort; see doc comment above
	return a
}

// RawService returns the underlying *ai.Service directly - the same
// singleton every setter in this file keeps in sync via loadAndApply, so
// a caller (IdeaService, and eventually AnalyticsService for Daily
// Review commentary) can hold onto this pointer for the lifetime of the
// app and always see the current configuration on its next Enhance call,
// with no extra database round-trip per use the way Service() below
// incurs. This is the accessor other services should actually use;
// Service() remains for a caller that specifically needs a guaranteed-
// fresh-from-disk read at that exact moment.
func (a *AIService) RawService() *ai.Service {
	return a.svc
}

// Service returns the underlying ai.Service, reloaded fresh from the
// database first. Prefer RawService for the common case (see its doc
// comment) - this exists for callers that need to be certain they're not
// seeing a config that's gone stale since the last setter call, at the
// cost of one extra database read.
func (a *AIService) Service() (*ai.Service, error) {
	if err := a.loadAndApply(); err != nil {
		return nil, err
	}
	return a.svc, nil
}

// GetConfig returns the currently saved AI configuration.
func (a *AIService) GetConfig() (AIConfigView, error) {
	cfg, err := a.store.Load(context.Background())
	if err != nil {
		return AIConfigView{}, fmt.Errorf("ai: loading config: %w", err)
	}
	return toView(cfg), nil
}

// SetEnabled turns AI on/off overall. Turning it off takes effect
// immediately for any call already in flight through a shared *ai.Service
// only up to Go's usual happens-before guarantees on the config mutex
// see ai.Service.SetConfig's own doc comment.
func (a *AIService) SetEnabled(enabled bool) error {
	if err := a.store.SetEnabled(context.Background(), enabled); err != nil {
		return fmt.Errorf("ai: saving enabled: %w", err)
	}
	return a.loadAndApply()
}

// SetProvider sets which local AI backend to use. Rejects anything that
// isn't a known Provider constant rather than silently storing garbage
// the Settings UI would then have no sensible way to render.
func (a *AIService) SetProvider(provider string) error {
	if !ai.ValidProvider(provider) {
		return fmt.Errorf("ai: invalid provider %q (want none, ollama, or openai_compatible)", provider)
	}
	if err := a.store.SetProvider(context.Background(), ai.Provider(provider)); err != nil {
		return fmt.Errorf("ai: saving provider: %w", err)
	}
	return a.loadAndApply()
}

// SetEndpoint sets the local server URL, e.g. "http://localhost:11434".
// Not validated as a well-formed URL here - Config.Validate does that at
// call time - so the person can type a partial value while still editing
// the field without every keystroke bouncing off an error.
func (a *AIService) SetEndpoint(endpoint string) error {
	if err := a.store.SetEndpoint(context.Background(), endpoint); err != nil {
		return fmt.Errorf("ai: saving endpoint: %w", err)
	}
	return a.loadAndApply()
}

// SetModel sets the model name/identifier to request, e.g. "llama3.1".
// Provider-specific and never validated against a list here - the local
// server is the only source of truth for which models it actually has.
func (a *AIService) SetModel(model string) error {
	if err := a.store.SetModel(context.Background(), model); err != nil {
		return fmt.Errorf("ai: saving model: %w", err)
	}
	return a.loadAndApply()
}

// SetScope turns one specific feature's AI permission on/off (section
// 71's "permissions / data scope" - section 37's "opt-in" applied
// per-feature, not just once globally). Rejects unknown scope strings.
func (a *AIService) SetScope(scope string, enabled bool) error {
	if !ai.ValidScope(scope) {
		return fmt.Errorf("ai: invalid scope %q", scope)
	}
	if err := a.store.SetScope(context.Background(), ai.Scope(scope), enabled); err != nil {
		return fmt.Errorf("ai: saving scope %q: %w", scope, err)
	}
	return a.loadAndApply()
}

// TestConnection sends a trivial prompt through the exact configuration
// currently saved and reports whether it worked - see ai.Service.
// TestConnection's own doc comment for why this deliberately reuses the
// real call path rather than a separate, simpler health check.
func (a *AIService) TestConnection() (AIResultView, error) {
	if err := a.loadAndApply(); err != nil {
		return AIResultView{}, fmt.Errorf("ai: loading config: %w", err)
	}
	result := a.svc.TestConnection(context.Background())
	return toResultView(result), nil
}

// Diagnostics returns non-sensitive information about the most recent
// AI call this run (spec section 55's Diagnostics page) - never the
// prompt or response text, which ai.LastCallInfo has no field for in the
// first place (see that type's own doc comment).
func (a *AIService) Diagnostics() AIDiagnosticsView {
	info := a.svc.LastCall()
	if info.At.IsZero() {
		return AIDiagnosticsView{HasRun: false}
	}
	return AIDiagnosticsView{
		HasRun:    true,
		At:        info.At.Format("2006-01-02T15:04:05Z07:00"),
		Scope:     string(info.Scope),
		Provider:  string(info.Provider),
		Success:   info.Success,
		Reason:    info.Reason,
		LatencyMs: info.Latency.Milliseconds(),
	}
}

func (a *AIService) loadAndApply() error {
	cfg, err := a.store.Load(context.Background())
	if err != nil {
		return fmt.Errorf("ai: loading config: %w", err)
	}
	a.svc.SetConfig(cfg)
	return nil
}

func toView(cfg ai.Config) AIConfigView {
	scopes := make(map[string]bool, len(ai.AllScopes))
	for _, sc := range ai.AllScopes {
		scopes[string(sc)] = cfg.Scopes[sc]
	}
	return AIConfigView{
		Enabled:  cfg.Enabled,
		Provider: string(cfg.Provider),
		Endpoint: cfg.Endpoint,
		Model:    cfg.Model,
		Scopes:   scopes,
	}
}

func toResultView(r ai.Result) AIResultView {
	return AIResultView{
		Text:        r.Text,
		Unavailable: r.Unavailable,
		Reason:      r.Reason,
		LatencyMs:   r.Latency.Milliseconds(),
	}
}
