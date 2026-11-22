package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAICompatibleClient_Generate_Success(t *testing.T) {
	var gotBody oaiChatRequest
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		resp := oaiChatResponse{}
		resp.Choices = []struct {
			Message oaiMessage `json:"message"`
		}{{Message: oaiMessage{Role: "assistant", Content: "hello from lm studio"}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := &OpenAICompatibleClient{Endpoint: srv.URL, Model: "qwen2.5-coder", Timeout: 5 * time.Second}
	text, err := c.Generate(context.Background(), Prompt{SystemPrompt: "be brief", UserPrompt: "say hi", MaxTokens: 64})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if text != "hello from lm studio" {
		t.Errorf("got text %q", text)
	}
	if gotBody.Model != "qwen2.5-coder" {
		t.Errorf("got model %q", gotBody.Model)
	}
	if gotBody.MaxTokens != 64 {
		t.Errorf("got max_tokens %d", gotBody.MaxTokens)
	}
	if len(gotBody.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(gotBody.Messages))
	}
	if gotAuth == "" {
		t.Errorf("expected an Authorization header to be sent (some local servers require the header present, even if unchecked)")
	}
}

func TestOpenAICompatibleClient_Generate_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := oaiChatResponse{Error: &struct {
			Message string `json:"message"`
		}{Message: "model not loaded"}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := &OpenAICompatibleClient{Endpoint: srv.URL, Model: "m"}
	_, err := c.Generate(context.Background(), Prompt{UserPrompt: "hi"})
	if err == nil || !strings.Contains(err.Error(), "model not loaded") {
		t.Fatalf("expected a 'model not loaded' error, got: %v", err)
	}
}

func TestOpenAICompatibleClient_Generate_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(oaiChatResponse{})
	}))
	defer srv.Close()

	c := &OpenAICompatibleClient{Endpoint: srv.URL, Model: "m"}
	_, err := c.Generate(context.Background(), Prompt{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected an error for zero choices")
	}
}

func TestOpenAICompatibleClient_Generate_ConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	c := &OpenAICompatibleClient{Endpoint: srv.URL, Model: "m", Timeout: 2 * time.Second}
	_, err := c.Generate(context.Background(), Prompt{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected a connection error")
	}
}
