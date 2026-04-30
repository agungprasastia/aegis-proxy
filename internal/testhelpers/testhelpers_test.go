package testhelpers

import (
	"net/http"
	"testing"
)

func TestSetupTestDB(t *testing.T) {
	db, cleanup := SetupTestDB()
	defer cleanup()

	if err := db.Ping(); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
}

func TestNewTestRequest(t *testing.T) {
	req := NewTestRequest(http.MethodPost, "/test", map[string]string{"name": "aegis"})
	if req.Method != http.MethodPost {
		t.Fatalf("method = %q, want %q", req.Method, http.MethodPost)
	}
	if req.URL.Path != "/test" {
		t.Fatalf("path = %q, want /test", req.URL.Path)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}

func TestAssertJSONFieldAndLoadFixture(t *testing.T) {
	body := LoadFixture("normalized_request.json")
	AssertJSONField(t, body, "model", "claude-sonnet-4.5")
}
