package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aegis-proxy/aegis/internal/database"
)

func TestOAuthCallbackStoresToken(t *testing.T) {
	db := testDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code_verifier") == "" {
			t.Fatalf("bad exchange form: %v", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-1", "refresh_token": "refresh-1", "expires_in": 3600})
	}))
	defer server.Close()

	engine := NewEngine(db, server.Client())
	cfg := config(server.URL)
	flow, err := engine.StartFlow(cfg, time.Minute)
	if err != nil {
		t.Fatalf("start flow: %v", err)
	}
	if !strings.Contains(flow.AuthorizeURL, "code_challenge_method=S256") || flow.Verifier == "" || flow.Challenge == "" {
		t.Fatalf("flow missing PKCE fields: %+v", flow)
	}

	token, err := engine.CompleteCallback(context.Background(), cfg, flow.State, "code-1")
	if err != nil {
		t.Fatalf("complete callback: %v", err)
	}
	if token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || token.ExpiresAt == nil {
		t.Fatalf("bad token: %+v", token)
	}
	stored, err := engine.LatestToken(context.Background(), "test")
	if err != nil {
		t.Fatalf("latest token: %v", err)
	}
	if stored.AccessToken != "access-1" {
		t.Fatalf("stored token mismatch: %+v", stored)
	}
}

func TestRefreshStoresNewToken(t *testing.T) {
	db := testDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "refresh-old" {
			t.Fatalf("bad refresh form: %v", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-new", "expires_in": 60})
	}))
	defer server.Close()

	engine := NewEngine(db, server.Client())
	token, err := engine.Refresh(context.Background(), config(server.URL), "refresh-old")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if token.AccessToken != "access-new" || token.RefreshToken != "refresh-old" {
		t.Fatalf("refresh token mismatch: %+v", token)
	}
}

func TestAbandonedFlowExpiresCleanly(t *testing.T) {
	engine := NewEngine(testDB(t), nil)
	cfg := config("https://example.invalid/token")
	flow, err := engine.StartFlow(cfg, time.Nanosecond)
	if err != nil {
		t.Fatalf("start flow: %v", err)
	}
	time.Sleep(time.Millisecond)
	_, err = engine.CompleteCallback(context.Background(), cfg, flow.State, "code")
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired flow error, got %v", err)
	}
}

func TestDisconnectRemovesProviderTokens(t *testing.T) {
	db := testDB(t)
	engine := NewEngine(db, nil)
	if err := engine.StoreToken(context.Background(), &Token{Provider: "test", AccessToken: "access", RefreshToken: "refresh"}); err != nil {
		t.Fatalf("store token: %v", err)
	}
	if err := engine.Disconnect(context.Background(), "test"); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if _, err := engine.LatestToken(context.Background(), "test"); err == nil {
		t.Fatal("expected token to be removed")
	}
}

func config(tokenURL string) ProviderConfig {
	return ProviderConfig{Provider: "test", ClientID: "client", RedirectURI: "http://127.0.0.1/callback", AuthURL: "https://example.com/auth", TokenURL: tokenURL, Scopes: []string{"read", "write"}}
}

func testDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "oauth.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
