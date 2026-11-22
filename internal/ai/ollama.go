package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaClient talks to Ollama's native chat API:
// POST {endpoint}/api/chat
// https://github.com/ollama/ollama/blob/main/docs/api.md#generate-a-chat-completion
//
// The native /api/chat route (rather than /api/generate) is used
// specifically because it accepts a system-role message natively, which
// maps directly onto Prompt.SystemPrompt without STELLAREI having to
// hand-roll prompt templating that would inevitably be tuned for one
// model family and wrong for another.
type OllamaClient struct {
	Endpoint string
	Model    string
	Timeout  time.Duration

	// httpClient is overridable in tests; nil means http.DefaultClient
	// with Timeout applied per-request via the context instead, so a
	// single shared client is safe across concurrent calls.
	httpClient *http.Client
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string                `json:"model"`
	Messages []ollamaChatMessage   `json:"messages"`
	Stream   bool                  `json:"stream"`
	Options  *ollamaChatReqOptions `json:"options,omitempty"`
}

type ollamaChatReqOptions struct {
	NumPredict int `json:"num_predict,omitempty"`
}

type ollamaChatResponse struct {
	Message ollamaChatMessage `json:"message"`
	Done    bool              `json:"done"`
	Error   string            `json:"error"`
}

func (c *OllamaClient) client() *http.Client {
	if c.httpClient != nil {
		return c.httpClient
	}
	return http.DefaultClient
}

func (c *OllamaClient) Generate(ctx context.Context, prompt Prompt) (string, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var messages []ollamaChatMessage
	if strings.TrimSpace(prompt.SystemPrompt) != "" {
		messages = append(messages, ollamaChatMessage{Role: "system", Content: prompt.SystemPrompt})
	}
	messages = append(messages, ollamaChatMessage{Role: "user", Content: prompt.UserPrompt})

	reqBody := ollamaChatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   false, // non-streaming keeps this one request/one response, matching the Client interface contract
	}
	if prompt.MaxTokens > 0 {
		reqBody.Options = &ollamaChatReqOptions{NumPredict: prompt.MaxTokens}
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ai/ollama: encoding request: %w", err)
	}

	url := strings.TrimRight(c.Endpoint, "/") + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ai/ollama: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client().Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ai/ollama: request to %s failed: %w", c.Endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4MB cap - a chat reply has no business being larger
	if err != nil {
		return "", fmt.Errorf("ai/ollama: reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ai/ollama: server returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed ollamaChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("ai/ollama: parsing response: %w", err)
	}
	if parsed.Error != "" {
		return "", fmt.Errorf("ai/ollama: %s", parsed.Error)
	}
	if strings.TrimSpace(parsed.Message.Content) == "" {
		return "", fmt.Errorf("ai/ollama: server returned an empty completion")
	}
	return parsed.Message.Content, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
