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

// OpenAICompatibleClient talks to any local server implementing the
// widely-adopted POST {endpoint}/v1/chat/completions shape - LM Studio's
// built-in server and llama.cpp's `llama-server` both do, unmodified.
// See ProviderOpenAICompatible's doc comment in types.go for why one
// client covers both of the spec's remaining named examples.
type OpenAICompatibleClient struct {
	Endpoint string
	Model    string
	Timeout  time.Duration

	httpClient *http.Client // overridable in tests; see OllamaClient
}

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiChatRequest struct {
	Model       string       `json:"model"`
	Messages    []oaiMessage `json:"messages"`
	Stream      bool         `json:"stream"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Temperature float64      `json:"temperature,omitempty"`
}

type oaiChatResponse struct {
	Choices []struct {
		Message oaiMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *OpenAICompatibleClient) client() *http.Client {
	if c.httpClient != nil {
		return c.httpClient
	}
	return http.DefaultClient
}

func (c *OpenAICompatibleClient) Generate(ctx context.Context, prompt Prompt) (string, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var messages []oaiMessage
	if strings.TrimSpace(prompt.SystemPrompt) != "" {
		messages = append(messages, oaiMessage{Role: "system", Content: prompt.SystemPrompt})
	}
	messages = append(messages, oaiMessage{Role: "user", Content: prompt.UserPrompt})

	reqBody := oaiChatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   false,
	}
	if prompt.MaxTokens > 0 {
		reqBody.MaxTokens = prompt.MaxTokens
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ai/openai_compatible: encoding request: %w", err)
	}

	url := strings.TrimRight(c.Endpoint, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ai/openai_compatible: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// Most local servers (LM Studio, llama.cpp) ignore auth entirely for
	// localhost connections; some accept any bearer token. Sending a
	// placeholder avoids servers that reject a totally missing header
	// without STELLAREI needing a separate "API key" field for what is,
	// for every documented use case here, not actually a secret.
	httpReq.Header.Set("Authorization", "Bearer not-needed")

	resp, err := c.client().Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ai/openai_compatible: request to %s failed: %w", c.Endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("ai/openai_compatible: reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ai/openai_compatible: server returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed oaiChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("ai/openai_compatible: parsing response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("ai/openai_compatible: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("ai/openai_compatible: server returned no completion choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
