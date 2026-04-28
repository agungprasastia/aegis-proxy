package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

type SessionData struct {
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]SessionData
}

func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]SessionData),
	}
	go sm.cleanupExpired()
	return sm
}

func (sm *SessionManager) Create() string {
	token := generateToken()
	now := time.Now()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sessions[token] = SessionData{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	}

	return token
}

func (sm *SessionManager) Validate(token string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[token]
	if !exists {
		return false
	}

	if time.Now().After(session.ExpiresAt) {
		return false
	}

	return true
}

func (sm *SessionManager) Delete(token string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, token)
}

func (sm *SessionManager) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for token, session := range sm.sessions {
			if now.After(session.ExpiresAt) {
				delete(sm.sessions, token)
			}
		}
		sm.mu.Unlock()
	}
}

func DashboardAuthMiddleware(sessions *SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/auth/login" {
				next.ServeHTTP(w, r)
				return
			}

			token := extractTokenFromRequest(r)
			if token == "" || !sessions.Validate(token) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"Unauthorized"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractTokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie("session_token"); err == nil {
		return cookie.Value
	}

	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}

	return ""
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
