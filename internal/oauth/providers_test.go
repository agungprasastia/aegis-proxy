package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaudeCodeProviderConfig(t *testing.T) {
	cfg, err := ProviderConfigFor(ProviderClaudeCode, "client", "http://127.0.0.1/callback")
	if err != nil {
		t.Fatalf("ProviderConfigFor: %v", err)
	}
	if cfg.Provider != ProviderClaudeCode || cfg.ClientID != "client" || !strings.Contains(strings.Join(cfg.Scopes, " "), "offline_access") {
		t.Fatalf("bad claude config: %+v", cfg)
	}
	spec, err := ProviderSpecFor(ProviderClaudeCode)
	if err != nil || spec.APIBaseURL == "" || spec.TokenHeader != "Authorization" {
		t.Fatalf("bad claude spec: %+v err=%v", spec, err)
	}
}

func TestOAuthProvidersRegisterThroughPKCE(t *testing.T) {
	providers := []string{ProviderCodex, ProviderGitHubCopilot, ProviderCursor}
	for _, name := range providers {
		t.Run(name, func(t *testing.T) {
			cfg, err := ProviderConfigFor(name, "client", "http://127.0.0.1/callback")
			if err != nil {
				t.Fatalf("config: %v", err)
			}
			engine := NewEngine(testDB(t), nil)
			flow, err := engine.StartFlow(cfg, 0)
			if err != nil {
				t.Fatalf("start flow: %v", err)
			}
			if flow.Provider != name || !strings.Contains(flow.AuthorizeURL, "code_challenge=") {
				t.Fatalf("flow missing provider/pkce: %+v", flow)
			}
		})
	}
}

func TestProviderRefreshIsolation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new-access", "refresh_token": "new-refresh"})
	}))
	defer server.Close()

	db := testDB(t)
	engine := NewEngine(db, server.Client())
	if err := engine.StoreToken(context.Background(), &Token{Provider: ProviderCodex, AccessToken: "codex-access", RefreshToken: "codex-refresh"}); err != nil {
		t.Fatalf("store codex: %v", err)
	}
	if err := engine.StoreToken(context.Background(), &Token{Provider: ProviderGitHubCopilot, AccessToken: "copilot-access", RefreshToken: "copilot-refresh"}); err != nil {
		t.Fatalf("store copilot: %v", err)
	}

	cfg := ProviderConfig{Provider: ProviderCursor, ClientID: "client", RedirectURI: "http://127.0.0.1/callback", AuthURL: "https://cursor.example/auth", TokenURL: server.URL}
	if _, err := engine.Refresh(context.Background(), cfg, "cursor-refresh"); err != nil {
		t.Fatalf("refresh cursor: %v", err)
	}
	codex, err := engine.LatestToken(context.Background(), ProviderCodex)
	if err != nil {
		t.Fatalf("latest codex: %v", err)
	}
	if codex.AccessToken != "codex-access" {
		t.Fatalf("codex token changed: %+v", codex)
	}
}
