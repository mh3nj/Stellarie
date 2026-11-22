package ai

import (
	"context"
	"sync"
	"time"
)

// Service is the one entry point the rest of STELLAREI should use for
// optional AI enhancement. It owns the scope-gating and graceful-
// degradation logic described in the package doc so that no caller
// (app/services/ai_service.go, and eventually the Daily Review / Pattern
// Discovery / "I HAVE NO IDEA" callers that opt into it) has to
// reimplement "is this actually allowed and available right now."
//
// Data-flow statement (spec section 37's disclosure requirement, and
// section 58's "explain exactly what is tracked" honesty principle
// applied to this feature too): the ONLY data that ever leaves the
// machine through this package is exactly what's in the Prompt passed to
// Enhance - assembled by the caller from that person's own STELLAREI
// data - sent to exactly one destination: Config.Endpoint, the local
// address the person typed into Settings themselves. Nothing here adds
// telemetry, nothing here has a hardcoded fallback endpoint, and nothing
// here retries against a different host if the configured one fails.
type Service struct {
	mu     sync.Mutex
	cfg    Config
	client Client // nil when AI is off or Provider is invalid; rebuilt by SetConfig

	newClient func(Config) (Client, error) // swappable in tests
	now       func() time.Time             // swappable in tests

	lastCall LastCallInfo
}

// LastCallInfo is what the Diagnostics page (spec section 55) may show.
// Deliberately excludes Prompt/response content - section 55: "Never
// log: ... personal data unnecessarily." Whether AI worked and how long
// it took is diagnostic; what was actually asked or answered is not
// this struct's business to retain.
type LastCallInfo struct {
	At       time.Time
	Scope    Scope
	Provider Provider
	Success  bool
	Reason   string // set when Success is false
	Latency  time.Duration
}

// NewService builds a Service for cfg. This never fails and never makes
// a network call - client construction is lazy and happens (or doesn't)
// per-call in Enhance, so that toggling AI off in Settings can't leave a
// stale client capable of firing a request after the fact.
func NewService(cfg Config) *Service {
	s := &Service{
		cfg:       cfg,
		newClient: NewClient,
		now:       time.Now,
	}
	return s
}

// SetConfig replaces the active configuration (called whenever the person
// changes an AI setting). Takes effect on the very next call - there is
// no cached client to invalidate because Enhance builds one fresh each
// time scope/enabled checks pass (see below); this keeps the "toggle off
// takes effect immediately" guarantee trivially true rather than
// depending on remembering to clear a cache.
func (s *Service) SetConfig(cfg Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}

// Config returns the currently active configuration.
func (s *Service) Config() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// LastCall returns diagnostic info about the most recent Enhance call, or
// the zero value if none has happened yet this run. Never persisted
// this is in-memory only and resets on restart, which is fine for a
// "what just happened" diagnostics panel and avoids ever writing prompt
// metadata to disk.
func (s *Service) LastCall() LastCallInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastCall
}

// Enhance is the single entry point for every optional-AI feature.
// It NEVER returns a Go error for "AI isn't available right now" in any
// of its forms (off, scope not permitted, unreachable, timed out, bad
// response) - those are all Result.Unavailable == true with a Reason
// safe to display. A non-nil error return is reserved for cases that
// indicate a bug in the calling code, not a runtime AI-availability
// condition, so that every caller can safely do:
//
//	result, err := aiService.Enhance(ctx, ai.ScopeDailyReviews, prompt)
//	if err != nil { return err } // programmer error, should never happen in normal operation
//	if result.Unavailable { /* fall back to the deterministic path - always keep working without AI */ }
func (s *Service) Enhance(ctx context.Context, scope Scope, prompt Prompt) (Result, error) {
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()

	if !cfg.ScopeAllowed(scope) {
		reason := "AI is turned off"
		if cfg.Enabled && cfg.Provider != ProviderNone {
			reason = "AI is on, but not enabled for this feature"
		}
		s.recordCall(scope, cfg.Provider, false, reason, 0)
		return Result{Unavailable: true, Reason: reason}, nil
	}

	client, err := s.buildClient(cfg)
	if err != nil {
		// A provider misconfiguration (e.g. saved before validation
		// existed, or an unknown value from a downgraded database) is
		// still an availability condition from the caller's point of
		// view, not a reason to hard-fail - section 56 again.
		reason := "AI provider is misconfigured: " + err.Error()
		s.recordCall(scope, cfg.Provider, false, reason, 0)
		return Result{Unavailable: true, Reason: reason}, nil
	}

	start := s.now()
	text, genErr := client.Generate(ctx, prompt)
	latency := s.now().Sub(start)

	if genErr != nil {
		reason := humanizeError(genErr)
		s.recordCall(scope, cfg.Provider, false, reason, latency)
		return Result{Unavailable: true, Reason: reason, Latency: latency}, nil
	}

	s.recordCall(scope, cfg.Provider, true, "", latency)
	return Result{Text: text, Latency: latency}, nil
}

// TestConnection is the backend for a Settings > AI "Test Connection"
// button (implied by section 71's AI settings needing to be verifiable
// before the person relies on them). It runs a trivial prompt through
// the exact same Enhance path as a real feature would, so "test
// connection succeeded" and "the actual feature will work" mean the same
// thing - there is no separate, simpler health-check route that could
// pass while a real prompt still fails.
func (s *Service) TestConnection(ctx context.Context) Result {
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()

	// Bypass scope-gating for the test itself - the person is testing
	// the CONNECTION, not any particular feature's permission, and may
	// not have enabled any scope yet. Enabled + a real provider is still
	// required; there is nothing to test with AI fully off.
	if !cfg.Enabled || cfg.Provider == ProviderNone {
		return Result{Unavailable: true, Reason: "AI is turned off"}
	}
	client, err := s.buildClient(cfg)
	if err != nil {
		return Result{Unavailable: true, Reason: "AI provider is misconfigured: " + err.Error()}
	}
	start := s.now()
	text, genErr := client.Generate(ctx, Prompt{
		UserPrompt: "Reply with the single word: OK",
		MaxTokens:  8,
	})
	latency := s.now().Sub(start)
	if genErr != nil {
		return Result{Unavailable: true, Reason: humanizeError(genErr), Latency: latency}
	}
	return Result{Text: text, Latency: latency}
}

func (s *Service) buildClient(cfg Config) (Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return s.newClient(cfg)
}

func (s *Service) recordCall(scope Scope, provider Provider, success bool, reason string, latency time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastCall = LastCallInfo{
		At:       s.now(),
		Scope:    scope,
		Provider: provider,
		Success:  success,
		Reason:   reason,
		Latency:  latency,
	}
}

// humanizeError turns a low-level client error into something safe and
// sensible to show directly in Settings/diagnostics, without leaking
// prompt content (the client errors in ollama.go/openai_compatible.go
// never include it) and without a raw Go error stack trace in the UI.
func humanizeError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
