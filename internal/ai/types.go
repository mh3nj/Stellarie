// Package ai implements STELLAREI's OPTIONAL local AI integration
// (spec section 37, roadmap Phase 11).
//
// Three things make this package different from most of the rest of the
// backend, and they're worth stating up front because they shape every
// decision below:
//
//  1. AI must NEVER be required for the core product (section 37,
//     restated in section 3's philosophy and section 56's error-handling
//     rules). internal/idea is deliberately AI-free and stays that way
//     this package is an entirely separate, additive layer that other
//     services may CALL OUT to, opt-in, never one that core features
//     depend on.
//  2. It talks to LOCAL endpoints the person runs themselves - Ollama,
//     LM Studio, or any llama.cpp-compatible server - never a cloud
//     provider. Section 37 is explicit that cloud AI, if it's ever added
//     at all, is a separate, later, opt-in-per-provider decision with its
//     own disclosure requirements. Nothing in this package makes an
//     outbound call anywhere except the single endpoint URL the person
//     typed into Settings themselves.
//  3. Every failure mode is a normal, expected outcome, not an error the
//     rest of the app has to handle specially. Section 56: "If AI
//     unavailable: STELLAREI must continue normally." So Service.Enhance
//     (service.go) never returns a Go error for "AI is off" / "this
//     feature isn't permitted to use AI" / "the local server didn't
//     answer" - those are all just Result.Unavailable == true with a
//     human-readable Reason. A Go `error` return is reserved for genuine
//     programmer mistakes (an invalid Config passed to NewService), not
//     for "the network call didn't work today."
package ai

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Provider identifies which local AI backend to talk to. There is
// deliberately no "cloud" provider here - see the package doc.
type Provider string

const (
	// ProviderNone means AI is off. NewService(cfg) with this provider
	// (or Config.Enabled == false) never constructs an HTTP client and
	// Service.Enhance always returns Result.Unavailable immediately,
	// without a network call.
	ProviderNone Provider = "none"

	// ProviderOllama talks to Ollama's native /api/chat endpoint
	// (https://github.com/ollama/ollama/blob/main/docs/api.md), the
	// default local install at http://localhost:11434.
	ProviderOllama Provider = "ollama"

	// ProviderOpenAICompatible talks to any server exposing an
	// OpenAI-style POST {endpoint}/v1/chat/completions route. This
	// deliberately covers two of the spec's three named examples with
	// one client: LM Studio's local server and llama.cpp's own
	// `llama-server` both implement this exact route, so there is no
	// product reason to hand-write three near-identical HTTP clients.
	// Anyone running a different OpenAI-compatible local server (vLLM,
	// text-generation-webui, etc.) also works under this provider.
	ProviderOpenAICompatible Provider = "openai_compatible"
)

func ValidProvider(p string) bool {
	switch Provider(p) {
	case ProviderNone, ProviderOllama, ProviderOpenAICompatible:
		return true
	default:
		return false
	}
}

// Scope is one specific feature that MAY be allowed to call out to the
// configured AI provider. Section 37's own list - "summaries, patterns,
// suggestions, project ideas, 'I HAVE NO IDEA', daily reviews" - is
// reproduced here almost verbatim, and section 71's Settings > AI section
// calls this "permissions / data scope": the person opts a feature in
// individually rather than one global switch turning on everything at
// once. Enabling AI overall (Config.Enabled) does nothing by itself until
// at least one Scope is also turned on.
type Scope string

const (
	ScopeSummaries    Scope = "summaries"      // Daily/Weekly Review narrative assistance
	ScopePatterns     Scope = "patterns"       // Pattern Discovery (section 31) phrasing help
	ScopeSuggestions  Scope = "suggestions"    // general suggestion text
	ScopeProjectIdeas Scope = "project_ideas"  // "I HAVE NO IDEA" → project generation (section 35)
	ScopeIHaveNoIdea  Scope = "i_have_no_idea" // "I HAVE NO IDEA" → the three suggestions (section 34)
	ScopeDailyReviews Scope = "daily_reviews"  // Time Receipt (section 60) commentary
)

// AllScopes lists every known scope, in the stable order the Settings UI
// should render its checkboxes.
var AllScopes = []Scope{
	ScopeSummaries,
	ScopePatterns,
	ScopeSuggestions,
	ScopeProjectIdeas,
	ScopeIHaveNoIdea,
	ScopeDailyReviews,
}

func ValidScope(s string) bool {
	for _, known := range AllScopes {
		if string(known) == s {
			return true
		}
	}
	return false
}

// Config is the fully-resolved AI configuration for one call. The
// service-layer binding (app/services/ai_service.go) is responsible for
// assembling this from the settings table; this package never reads a
// database itself, matching the plain-Go-package convention the rest of
// internal/ already follows (see internal/closebehavior).
type Config struct {
	Enabled  bool
	Provider Provider
	Endpoint string // e.g. "http://localhost:11434" or "http://127.0.0.1:1234"
	Model    string // e.g. "llama3.1", "qwen2.5-coder" - provider-specific, never validated here
	Scopes   map[Scope]bool
	// Timeout bounds every outbound call. Zero means DefaultTimeout.
	// Local inference can legitimately take a while on modest hardware,
	// but STELLAREI must never let a slow/hung local server block a UI
	// action indefinitely (section 56's "must continue normally" implies
	// "must not hang").
	Timeout time.Duration
}

// DefaultTimeout is generous relative to a typical cloud API call on
// purpose - local inference on CPU-only or modest consumer hardware can
// take tens of seconds for a single completion, and a premature timeout
// here would look indistinguishable from "server unreachable" to the
// person configuring this.
const DefaultTimeout = 45 * time.Second

// ScopeAllowed reports whether cfg permits scope to call out to AI at
// all - i.e. both the overall switch and the specific per-feature
// checkbox are on. Every caller into this package's Service goes through
// this exact check (service.go), so it's exposed here for the Settings
// UI / diagnostics to reuse the identical logic rather than reimplement
// it slightly differently.
func (c Config) ScopeAllowed(scope Scope) bool {
	if !c.Enabled || c.Provider == ProviderNone || c.Provider == "" {
		return false
	}
	return c.Scopes[scope]
}

// effectiveTimeout returns c.Timeout, or DefaultTimeout if unset.
func (c Config) effectiveTimeout() time.Duration {
	if c.Timeout <= 0 {
		return DefaultTimeout
	}
	return c.Timeout
}

// Validate checks that cfg is internally consistent enough to attempt a
// call. It does NOT check reachability - that's what TestConnection
// (service.go) is for, and it's a genuinely separate question ("is this
// configured sensibly" vs. "is the server actually up right now").
func (c Config) Validate() error {
	if !c.Enabled || c.Provider == ProviderNone {
		return nil // nothing to validate when AI is off
	}
	if !ValidProvider(string(c.Provider)) {
		return fmt.Errorf("ai: unknown provider %q", c.Provider)
	}
	if strings.TrimSpace(c.Endpoint) == "" {
		return fmt.Errorf("ai: endpoint is required when provider is %q", c.Provider)
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("ai: endpoint %q is not a valid http(s) URL", c.Endpoint)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("ai: endpoint %q must be http or https", c.Endpoint)
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("ai: model is required when provider is %q", c.Provider)
	}
	return nil
}

// IsLocalHost reports whether endpoint's host looks like this machine
// (localhost / 127.0.0.1 / ::1). This is informational only - used by the
// Settings UI to show a "local" badge (or, per spec section 37's
// disclosure requirement for anything that isn't guaranteed local, the
// absence of one) - and is never used to block a request. Someone
// running Ollama on another machine on their own LAN is still using
// local-network-only AI in the spirit of section 37; STELLAREI just
// can't label that case "local" with full confidence.
func IsLocalHost(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// Prompt is one request to generate text. SystemPrompt sets the AI's
// role/constraints; UserPrompt is the actual content-bearing request
// (e.g. "Summarize this week: <real session data the caller assembled>").
// This package never invents either field - every byte sent to the local
// server comes from the caller (app/services/ai_service.go), which in
// turn only sources it from data the person's own STELLAREI database
// already holds. See service.go's doc comment for the full data-flow
// statement.
type Prompt struct {
	SystemPrompt string
	UserPrompt   string
	// MaxTokens caps the response length. 0 lets the provider use its own
	// default rather than this package guessing a "reasonable" number
	// that might not fit every model.
	MaxTokens int
}

// Result is what Service.Enhance always returns instead of a Go error
// for anything short of a programming mistake - see the package doc.
type Result struct {
	Text        string
	Unavailable bool   // true if no text was produced, for ANY reason
	Reason      string // human-readable, safe to show directly in the UI; never includes prompt/response content
	Latency     time.Duration
}
