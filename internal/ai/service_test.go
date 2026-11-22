package ai

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeClient lets service_test.go assert exactly how many times (and
// with what) Generate was called, and inject any error/text it wants
// without spinning up real HTTP servers for tests that are really about
// Service's own gating logic, not the wire protocol (that's ollama_test.go
// and openai_compatible_test.go's job).
type fakeClient struct {
	calls int
	text  string
	err   error
}

func (f *fakeClient) Generate(ctx context.Context, prompt Prompt) (string, error) {
	f.calls++
	return f.text, f.err
}

func newTestService(cfg Config, fc *fakeClient) *Service {
	s := NewService(cfg)
	s.newClient = func(Config) (Client, error) { return fc, nil }
	return s
}

func TestService_Enhance_DisabledByDefault(t *testing.T) {
	// The zero value of Config (Enabled: false) is exactly what a fresh
	// install has before the person ever visits Settings > AI - section
	// 37: "disabled by default". This test exists specifically to pin
	// that default.
	fc := &fakeClient{text: "should never see this"}
	s := newTestService(Config{}, fc)

	result, err := s.Enhance(context.Background(), ScopeDailyReviews, Prompt{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("Enhance returned a Go error for the disabled case: %v", err)
	}
	if !result.Unavailable {
		t.Fatal("expected Result.Unavailable when AI is disabled")
	}
	if fc.calls != 0 {
		t.Fatalf("expected zero network-layer calls when AI is disabled, got %d", fc.calls)
	}
}

func TestService_Enhance_EnabledButScopeNotPermitted(t *testing.T) {
	cfg := Config{
		Enabled:  true,
		Provider: ProviderOllama,
		Endpoint: "http://localhost:11434",
		Model:    "llama3.1",
		Scopes: map[Scope]bool{
			ScopeSummaries: true, // Daily Reviews is NOT in this set
		},
	}
	fc := &fakeClient{text: "should never see this"}
	s := newTestService(cfg, fc)

	result, err := s.Enhance(context.Background(), ScopeDailyReviews, Prompt{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.Unavailable {
		t.Fatal("expected Result.Unavailable when the specific scope isn't permitted")
	}
	if fc.calls != 0 {
		t.Fatalf("expected zero calls when scope isn't permitted, got %d", fc.calls)
	}
	if result.Reason == "" {
		t.Error("expected a human-readable Reason")
	}
}

func TestService_Enhance_EnabledAndPermitted_Success(t *testing.T) {
	cfg := Config{
		Enabled:  true,
		Provider: ProviderOllama,
		Endpoint: "http://localhost:11434",
		Model:    "llama3.1",
		Scopes:   map[Scope]bool{ScopeDailyReviews: true},
	}
	fc := &fakeClient{text: "here is your summary"}
	s := newTestService(cfg, fc)

	result, err := s.Enhance(context.Background(), ScopeDailyReviews, Prompt{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.Unavailable {
		t.Fatalf("expected success, got Unavailable: %s", result.Reason)
	}
	if result.Text != "here is your summary" {
		t.Errorf("got text %q", result.Text)
	}
	if fc.calls != 1 {
		t.Errorf("expected exactly 1 call, got %d", fc.calls)
	}
}

func TestService_Enhance_ClientErrorDegradesGracefully(t *testing.T) {
	cfg := Config{
		Enabled:  true,
		Provider: ProviderOllama,
		Endpoint: "http://localhost:11434",
		Model:    "llama3.1",
		Scopes:   map[Scope]bool{ScopeIHaveNoIdea: true},
	}
	fc := &fakeClient{err: errors.New("connection refused")}
	s := newTestService(cfg, fc)

	result, err := s.Enhance(context.Background(), ScopeIHaveNoIdea, Prompt{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("a local-server failure must not become a Go error (section 56): %v", err)
	}
	if !result.Unavailable {
		t.Fatal("expected Result.Unavailable when the local server errors")
	}
	if result.Reason == "" {
		t.Error("expected a Reason describing the failure")
	}
}

func TestService_Enhance_InvalidConfigDegradesGracefully(t *testing.T) {
	cfg := Config{
		Enabled:  true,
		Provider: ProviderOllama,
		Endpoint: "", // invalid: empty endpoint
		Model:    "llama3.1",
		Scopes:   map[Scope]bool{ScopeSummaries: true},
	}
	s := NewService(cfg) // real newClient - must not even reach it, Validate should short-circuit first

	result, err := s.Enhance(context.Background(), ScopeSummaries, Prompt{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("a misconfiguration must degrade gracefully, not become a Go error: %v", err)
	}
	if !result.Unavailable {
		t.Fatal("expected Unavailable for an invalid config")
	}
}

func TestService_SetConfig_TakesEffectImmediately(t *testing.T) {
	fc := &fakeClient{text: "ok"}
	s := newTestService(Config{}, fc) // starts disabled

	result, _ := s.Enhance(context.Background(), ScopeSummaries, Prompt{UserPrompt: "hi"})
	if !result.Unavailable {
		t.Fatal("expected disabled at the start")
	}

	s.SetConfig(Config{
		Enabled:  true,
		Provider: ProviderOllama,
		Endpoint: "http://localhost:11434",
		Model:    "llama3.1",
		Scopes:   map[Scope]bool{ScopeSummaries: true},
	})

	result, err := s.Enhance(context.Background(), ScopeSummaries, Prompt{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error after SetConfig: %v", err)
	}
	if result.Unavailable {
		t.Fatalf("expected success after enabling via SetConfig, got: %s", result.Reason)
	}
}

func TestService_LastCall_RecordsOutcomeWithoutPromptContent(t *testing.T) {
	cfg := Config{
		Enabled:  true,
		Provider: ProviderOllama,
		Endpoint: "http://localhost:11434",
		Model:    "llama3.1",
		Scopes:   map[Scope]bool{ScopeSummaries: true},
	}
	fc := &fakeClient{text: "a secret-looking reply the diagnostics panel must never surface"}
	s := newTestService(cfg, fc)
	s.now = func() time.Time { return time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC) }

	_, _ = s.Enhance(context.Background(), ScopeSummaries, Prompt{UserPrompt: "hi"})

	info := s.LastCall()
	if !info.Success {
		t.Fatal("expected Success == true")
	}
	if info.Scope != ScopeSummaries {
		t.Errorf("got scope %q", info.Scope)
	}
	if info.Provider != ProviderOllama {
		t.Errorf("got provider %q", info.Provider)
	}
	// LastCallInfo has no field capable of holding prompt/response text
	// this assertion exists so that if one is ever added, it fails loudly
	// rather than silently starting to leak content into diagnostics.
	if info.Reason != "" {
		t.Errorf("expected empty Reason on success, got %q", info.Reason)
	}
}

func TestService_TestConnection_OffByDefault(t *testing.T) {
	fc := &fakeClient{text: "OK"}
	s := newTestService(Config{}, fc)

	result := s.TestConnection(context.Background())
	if !result.Unavailable {
		t.Fatal("expected Unavailable when AI is off")
	}
	if fc.calls != 0 {
		t.Errorf("expected zero calls, got %d", fc.calls)
	}
}

func TestService_TestConnection_IgnoresScopesButRequiresEnabled(t *testing.T) {
	cfg := Config{
		Enabled:  true,
		Provider: ProviderOllama,
		Endpoint: "http://localhost:11434",
		Model:    "llama3.1",
		Scopes:   map[Scope]bool{}, // nothing opted in - TestConnection should still run
	}
	fc := &fakeClient{text: "OK"}
	s := newTestService(cfg, fc)

	result := s.TestConnection(context.Background())
	if result.Unavailable {
		t.Fatalf("TestConnection should not require a scope to be enabled, got: %s", result.Reason)
	}
	if fc.calls != 1 {
		t.Errorf("expected exactly 1 call, got %d", fc.calls)
	}
}

func TestScopeAllowed(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"zero value", Config{}, false},
		{"enabled but provider none", Config{Enabled: true, Provider: ProviderNone, Scopes: map[Scope]bool{ScopeSummaries: true}}, false},
		{"enabled, provider set, scope off", Config{Enabled: true, Provider: ProviderOllama, Scopes: map[Scope]bool{}}, false},
		{"enabled, provider set, scope on", Config{Enabled: true, Provider: ProviderOllama, Scopes: map[Scope]bool{ScopeSummaries: true}}, true},
		{"disabled but scope on", Config{Enabled: false, Provider: ProviderOllama, Scopes: map[Scope]bool{ScopeSummaries: true}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.ScopeAllowed(ScopeSummaries); got != tc.want {
				t.Errorf("ScopeAllowed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"disabled - always valid", Config{Enabled: false, Provider: "garbage"}, false},
		{"none provider - always valid", Config{Enabled: true, Provider: ProviderNone}, false},
		{"valid ollama", Config{Enabled: true, Provider: ProviderOllama, Endpoint: "http://localhost:11434", Model: "llama3.1"}, false},
		{"missing endpoint", Config{Enabled: true, Provider: ProviderOllama, Model: "llama3.1"}, true},
		{"missing model", Config{Enabled: true, Provider: ProviderOllama, Endpoint: "http://localhost:11434"}, true},
		{"unknown provider", Config{Enabled: true, Provider: "made-up", Endpoint: "http://localhost:1", Model: "m"}, true},
		{"non-http scheme", Config{Enabled: true, Provider: ProviderOllama, Endpoint: "ftp://localhost:1", Model: "m"}, true},
		{"unparseable endpoint", Config{Enabled: true, Provider: ProviderOllama, Endpoint: "://::not a url", Model: "m"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestIsLocalHost(t *testing.T) {
	cases := map[string]bool{
		"http://localhost:11434":     true,
		"http://127.0.0.1:1234":      true,
		"http://[::1]:8080":          true,
		"http://192.168.1.50:11434":  false,
		"http://my-server.lan:11434": false,
		"not a url":                  false,
	}
	for endpoint, want := range cases {
		if got := IsLocalHost(endpoint); got != want {
			t.Errorf("IsLocalHost(%q) = %v, want %v", endpoint, got, want)
		}
	}
}
