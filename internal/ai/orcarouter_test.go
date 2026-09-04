package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseSuggestion(t *testing.T) {
	got, err := parseSuggestion("```json\n{\"type\":\"Feat\",\"scope\":\" auth \",\"subject\":\"add login\",\"body\":\"persist session\"}\n```")
	if err != nil {
		t.Fatalf("parseSuggestion: %v", err)
	}
	if got.Type != "feat" || got.Scope != "auth" || got.Subject != "add login" || got.Body != "persist session" {
		t.Fatalf("unexpected suggestion: %#v", got)
	}
}

func TestGenerateCommitSuggestion(t *testing.T) {
	var receivedModel string
	var receivedAuth string
	var receivedUserContent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		receivedAuth = r.Header.Get("Authorization")

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		receivedModel = req.Model
		if len(req.Messages) > 1 {
			receivedUserContent = req.Messages[1].Content
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"type\":\"fix\",\"scope\":\"commit\",\"subject\":\"handle staged diff\",\"body\":\"\"}"}}]}`))
	}))
	defer server.Close()

	client := &OrcaRouterClient{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		Model:      "test-model",
		HTTPClient: server.Client(),
	}
	got, err := client.GenerateCommitSuggestion(context.Background(), "diff --git a/a b/a\n+hello")
	if err != nil {
		t.Fatalf("GenerateCommitSuggestion: %v", err)
	}
	if got.Type != "fix" || got.Scope != "commit" || got.Subject != "handle staged diff" {
		t.Fatalf("unexpected suggestion: %#v", got)
	}
	if receivedAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", receivedAuth)
	}
	if receivedModel != "test-model" {
		t.Fatalf("model = %q", receivedModel)
	}
	if !strings.Contains(receivedUserContent, "+hello") {
		t.Fatalf("staged diff was not included in prompt")
	}
}

func TestGenerateCommitSuggestionRejectsEmptyDiff(t *testing.T) {
	client := &OrcaRouterClient{APIKey: "test", BaseURL: "http://example.invalid", Model: "test"}
	if _, err := client.GenerateCommitSuggestion(context.Background(), "   "); err == nil {
		t.Fatal("expected empty staged diff error")
	}
}
