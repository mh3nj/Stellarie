package ai

import "context"

// Client is the narrow interface both provider implementations satisfy.
// Service (service.go) only ever talks to this interface, never to
// *OllamaClient / *OpenAICompatibleClient directly - that's what makes
// service_test.go able to exercise the disabled/scope-denied/timeout
// paths with a fake Client instead of a real HTTP server.
type Client interface {
	// Generate sends prompt to the local server and returns the model's
	// reply text. Implementations must respect ctx's deadline and must
	// never retry silently - one call in, one HTTP request out, so
	// latency stays predictable and testable.
	Generate(ctx context.Context, prompt Prompt) (string, error)
}

// NewClient builds the Client for cfg.Provider. Returns an error only for
// ProviderNone or an unknown provider - callers should already have
// checked Config.ScopeAllowed before reaching here (Service.Enhance
// does), so in normal operation this is only ever called with a real
// provider.
func NewClient(cfg Config) (Client, error) {
	switch cfg.Provider {
	case ProviderOllama:
		return &OllamaClient{Endpoint: cfg.Endpoint, Model: cfg.Model, Timeout: cfg.effectiveTimeout()}, nil
	case ProviderOpenAICompatible:
		return &OpenAICompatibleClient{Endpoint: cfg.Endpoint, Model: cfg.Model, Timeout: cfg.effectiveTimeout()}, nil
	default:
		return nil, &unknownProviderError{provider: cfg.Provider}
	}
}

type unknownProviderError struct{ provider Provider }

func (e *unknownProviderError) Error() string {
	return "ai: cannot build a client for provider " + string(e.provider)
}
