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

func TestOllamaClient_Generate_Success(t *testing.T) {
	var gotBody ollamaChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ollamaChatResponse{
			Message: ollamaChatMessage{Role: "assistant", Content: "hello from ollama"},
			Done:    true,
		})
	}))
	defer srv.Close()

	c := &OllamaClient{Endpoint: srv.URL, Model: "llama3.1", Timeout: 5 * time.Second}
	text, err := c.Generate(context.Background(), Prompt{
		SystemPrompt: "be brief",
		UserPrompt:   "say hi",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if text != "hello from ollama" {
		t.Errorf("got text %q", text)
	}

	if gotBody.Model != "llama3.1" {
		t.Errorf("got model %q", gotBody.Model)
	}
	if gotBody.Stream {
		t.Errorf("expected stream=false")
	}
	if len(gotBody.Messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(gotBody.Messages))
	}
	if gotBody.Messages[0].Role != "system" || gotBody.Messages[0].Content != "be brief" {
		t.Errorf("unexpected system message: %+v", gotBody.Messages[0])
	}
	if gotBody.Messages[1].Role != "user" || gotBody.Messages[1].Content != "say hi" {
		t.Errorf("unexpected user message: %+v", gotBody.Messages[1])
	}
}

func TestOllamaClient_Generate_NoSystemPrompt(t *testing.T) {
	var gotBody ollamaChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(ollamaChatResponse{Message: ollamaChatMessage{Content: "ok"}})
	}))
	defer srv.Close()

	c := &OllamaClient{Endpoint: srv.URL, Model: "m"}
	_, err := c.Generate(context.Background(), Prompt{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if len(gotBody.Messages) != 1 {
		t.Fatalf("expected exactly 1 message when SystemPrompt is empty, got %d", len(gotBody.Messages))
	}
}

func TestOllamaClient_Generate_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	c := &OllamaClient{Endpoint: srv.URL, Model: "m"}
	_, err := c.Generate(context.Background(), Prompt{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected an error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention the status code, got: %v", err)
	}
}

func TestOllamaClient_Generate_OllamaLevelError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ollamaChatResponse{Error: "model 'llama3.1' not found"})
	}))
	defer srv.Close()

	c := &OllamaClient{Endpoint: srv.URL, Model: "llama3.1"}
	_, err := c.Generate(context.Background(), Prompt{UserPrompt: "hi"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected a 'not found' error, got: %v", err)
	}
}

func TestOllamaClient_Generate_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := &OllamaClient{Endpoint: srv.URL, Model: "m"}
	_, err := c.Generate(context.Background(), Prompt{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestOllamaClient_Generate_ConnectionRefused(t *testing.T) {
	// A closed server: guaranteed nothing is listening.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	c := &OllamaClient{Endpoint: srv.URL, Model: "m", Timeout: 2 * time.Second}
	_, err := c.Generate(context.Background(), Prompt{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected a connection error when nothing is listening")
	}
}

func TestOllamaClient_Generate_RespectsTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(ollamaChatResponse{Message: ollamaChatMessage{Content: "too slow"}})
	}))
	defer srv.Close()

	c := &OllamaClient{Endpoint: srv.URL, Model: "m", Timeout: 20 * time.Millisecond}
	_, err := c.Generate(context.Background(), Prompt{UserPrompt: "hi"})
	if err == nil {
		t.Fatal("expected a timeout error")
	}
}
