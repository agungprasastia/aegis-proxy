package anthropic

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
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "valid-key" {
			t.Fatalf("missing api key header")
		}
		if r.Header.Get("anthropic-version") != anthropicVersionHeader {
			t.Fatalf("missing anthropic version header")
		}

		var req messageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "claude-3-5-sonnet-latest" || len(req.Messages) != 1 || req.Messages[0].Content != "hello" {
			t.Fatalf("unexpected request: %#v", req)
		}

		json.NewEncoder(w).Encode(messageResponse{
			Content:    []contentBlock{{Type: "text", Text: "world"}},
			StopReason: "end_turn",
			Usage:      usage{InputTokens: 2, OutputTokens: 3},
		})
	}))
	defer server.Close()

	p := NewAnthropicProvider(server.URL)
	resp, err := p.SendChatCompletion(context.Background(), account("valid-key"), &provider.ChatRequest{
		Model:     "claude-3-5-sonnet-latest",
		Messages:  []provider.ChatMessage{{Role: "user", Content: "hello"}},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("SendChatCompletion failed: %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "world" || resp.Choices[0].FinishReason != "stop" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %#v", resp.Usage)
	}
}

func TestInvalidKeyReturnsNormalizedAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"type":"authentication_error"}}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	p := NewAnthropicProvider(server.URL)
	_, err := p.SendChatCompletion(context.Background(), account("bad-key"), &provider.ChatRequest{
		Model:    "claude-3-5-sonnet-latest",
		Messages: []provider.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Fatal("invalid key succeeded")
	}
	if !strings.Contains(err.Error(), "authentication failed: invalid or expired api_key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModelListingWorks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "valid-key" {
			t.Fatalf("missing api key header")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{{"id": "claude-3-5-sonnet-latest"}, {"id": "claude-3-haiku-20240307"}},
		})
	}))
	defer server.Close()

	p := NewAnthropicProvider(server.URL)
	models, err := p.ListModels(context.Background(), account("valid-key"))
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 2 || models[0] != "claude-3-5-sonnet-latest" || models[1] != "claude-3-haiku-20240307" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func account(apiKey string) *models.Account {
	return &models.Account{Token: `{"api_key":"` + apiKey + `"}`}
}
