package apikey

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aegis-proxy/aegis/internal/models"
	_ "modernc.org/sqlite"
)

func TestConnectivityValidInvalidKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]string{{"id": "model-a"}}})
	}))
	defer server.Close()

	provider := NewAPIKeyProvider("test", server.URL, "Authorization")
	if err := provider.TestConnectivity(context.Background(), "valid-key"); err != nil {
		t.Fatalf("valid key failed: %v", err)
	}

	provider.Models = nil
	if err := provider.TestConnectivity(context.Background(), "invalid-key"); err == nil {
		t.Fatal("invalid key succeeded")
	}
}

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]string{{"id": "alpha"}, {"id": "beta"}}})
	}))
	defer server.Close()

	provider := NewAPIKeyProvider("test", server.URL, "x-api-key")
	models, err := provider.ListModels(context.Background(), "key")
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 2 || models[0] != "alpha" || models[1] != "beta" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestSendRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "route-key" {
			t.Fatalf("missing api key header")
		}
		var req NormalizedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "alpha" || len(req.Messages) != 1 || req.Messages[0].Content != "hello" {
			t.Fatalf("unexpected request: %#v", req)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message":       map[string]string{"content": "world"},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3},
		})
	}))
	defer server.Close()

	provider := NewAPIKeyProvider("test", server.URL, "x-api-key")
	resp, err := provider.SendRequest(context.Background(), "route-key", &NormalizedRequest{
		Model:    "alpha",
		Messages: []models.NormalizedMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("SendRequest failed: %v", err)
	}
	if resp.Content != "world" || resp.FinishReason != models.FinishReasonStop || resp.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestAPIKeyStorageEncryptsKeys(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE api_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider TEXT NOT NULL,
		key TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	if err := SaveAPIKey(db, "test", "secret-key"); err != nil {
		t.Fatalf("SaveAPIKey failed: %v", err)
	}

	var stored string
	if err := db.QueryRow("SELECT key FROM api_keys WHERE provider = ?", "test").Scan(&stored); err != nil {
		t.Fatalf("read stored key: %v", err)
	}
	if stored == "secret-key" || strings.Contains(stored, "secret-key") {
		t.Fatalf("key stored unencrypted: %q", stored)
	}

	key, err := GetAPIKey(db, "test")
	if err != nil {
		t.Fatalf("GetAPIKey failed: %v", err)
	}
	if key != "secret-key" {
		t.Fatalf("unexpected decrypted key: %q", key)
	}
}
