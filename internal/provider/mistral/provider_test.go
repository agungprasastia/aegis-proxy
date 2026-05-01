package mistral

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
)

func TestProviderAdapter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/models":
			json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]string{{"id": "mistral-large-latest"}}})
		case "/v1/chat/completions":
			json.NewEncoder(w).Encode(map[string]interface{}{"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	p := NewProvider(server.URL)
	account := &models.Account{Token: "test-key"}
	if err := p.ValidateAccount(context.Background(), account); err != nil {
		t.Fatalf("ValidateAccount failed: %v", err)
	}
	resp, err := p.SendChatCompletion(context.Background(), account, &provider.ChatRequest{Model: "mistral-large", Messages: []provider.ChatMessage{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("SendChatCompletion failed: %v", err)
	}
	if resp.Choices[0].Message.Content != "ok" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if !p.SupportsModel("codestral") {
		t.Fatal("expected codestral support")
	}
}

func TestProviderErrorNormalization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "bad key", http.StatusUnauthorized) }))
	defer server.Close()
	err := NewProvider(server.URL).ValidateAccount(context.Background(), &models.Account{Token: "bad"})
	if err == nil || !strings.Contains(err.Error(), "mistral authentication failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
