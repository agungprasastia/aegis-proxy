package gemini

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
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer valid-key" {
			t.Fatalf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		var req models.NormalizedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "gemini-2.5-flash" || len(req.Messages) != 1 || req.Messages[0].Content != "hello" {
			t.Fatalf("unexpected request: %#v", req)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message":       map[string]string{"content": "hi"},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3},
		})
	}))
	defer server.Close()

	p := NewGeminiProvider(server.URL)
	resp, err := p.SendChatCompletion(context.Background(), &models.Account{Token: "valid-key"}, &provider.ChatRequest{
		Model:    "gemini-2.5-flash",
		Messages: []provider.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("SendChatCompletion failed: %v", err)
	}
	if resp.Choices[0].Message.Content != "hi" || resp.Choices[0].FinishReason != "stop" || resp.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestInvalidKeyReturnsNormalizedAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "API key not valid", http.StatusUnauthorized)
	}))
	defer server.Close()

	p := NewGeminiProvider(server.URL)
	_, err := p.SendChatCompletion(context.Background(), &models.Account{Token: "invalid-key"}, &provider.ChatRequest{
		Model:    "gemini-2.5-flash",
		Messages: []provider.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "authentication failed: invalid or expired api_key" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnsupportedMultimodalFieldsRejected(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewGeminiProvider(server.URL)
	_, err := p.SendChatCompletion(context.Background(), &models.Account{Token: "valid-key"}, &provider.ChatRequest{
		Model:    "gemini-2.5-flash",
		Messages: []provider.ChatMessage{{Role: "user", Content: `[{"type":"image_url","image_url":{"url":"data:image/png;base64,abc"}}]`}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "multimodal fields") {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("upstream called for unsupported multimodal request")
	}
}
