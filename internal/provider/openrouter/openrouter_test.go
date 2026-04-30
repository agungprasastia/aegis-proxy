package openrouter

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

func TestValidAPIKeyRoutesSuccessfully(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer valid-key" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message":       map[string]string{"content": "pong"},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3},
		})
	}))
	defer server.Close()

	p := NewOpenRouterProviderWithBaseURL(server.URL)
	resp, err := p.SendChatCompletion(context.Background(), account("valid-key"), &provider.ChatRequest{
		Model:    "openai/gpt-4o-mini",
		Messages: []provider.ChatMessage{{Role: "user", Content: "ping"}},
	})
	if err != nil {
		t.Fatalf("SendChatCompletion failed: %v", err)
	}
	if resp.Choices[0].Message.Content != "pong" || resp.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestInvalidKeyReturnsNormalizedAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	p := NewOpenRouterProviderWithBaseURL(server.URL)
	_, err := p.SendChatCompletion(context.Background(), account("invalid-key"), &provider.ChatRequest{
		Model:    "openai/gpt-4o-mini",
		Messages: []provider.ChatMessage{{Role: "user", Content: "ping"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "authentication failed: invalid or expired api_key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModelPassThroughWorks(t *testing.T) {
	var routedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		routedModel = req.Model
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message":       map[string]string{"content": "ok"},
				"finish_reason": "stop",
			}},
		})
	}))
	defer server.Close()

	p := NewOpenRouterProviderWithBaseURL(server.URL)
	modelID := "anthropic/claude-3.5-sonnet"
	if !p.SupportsModel(modelID) || p.MapModel(modelID) != modelID {
		t.Fatalf("model pass-through flags failed")
	}
	if _, err := p.SendChatCompletion(context.Background(), account("valid-key"), &provider.ChatRequest{Model: modelID}); err != nil {
		t.Fatalf("SendChatCompletion failed: %v", err)
	}
	if routedModel != modelID {
		t.Fatalf("model was not passed through: %q", routedModel)
	}
}

func account(apiKey string) *models.Account {
	return &models.Account{Token: `{"api_key":"` + apiKey + `"}`}
}
