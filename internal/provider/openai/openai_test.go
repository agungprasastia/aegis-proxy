package openai

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

func TestSendChatCompletionValidAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer valid-key" {
			t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}

		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "gpt-test" || len(req.Messages) != 1 || req.Messages[0].Content != "hello" {
			t.Fatalf("unexpected request: %#v", req)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message":       map[string]string{"content": "world"},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider(server.URL)
	resp, err := p.SendChatCompletion(context.Background(), &models.Account{Token: "valid-key"}, &provider.ChatRequest{
		Model:    "gpt-test",
		Messages: []provider.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("SendChatCompletion failed: %v", err)
	}
	if resp.Model != "gpt-test" || resp.Choices[0].Message.Content != "world" || resp.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestSendChatCompletionInvalidAPIKeyReturnsNormalizedAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"Incorrect API key provided"}}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	p := NewOpenAIProvider(server.URL)
	_, err := p.SendChatCompletion(context.Background(), &models.Account{Token: "invalid-key"}, &provider.ChatRequest{Model: "gpt-test"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "authentication failed: invalid or expired api_key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer valid-key" {
			t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{{"id": "gpt-a"}, {"id": "gpt-b"}},
		})
	}))
	defer server.Close()

	p := NewOpenAIProvider(server.URL)
	models, err := p.ListModels(context.Background(), &models.Account{Token: "valid-key"})
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 2 || models[0] != "gpt-a" || models[1] != "gpt-b" {
		t.Fatalf("unexpected models: %#v", models)
	}
}
