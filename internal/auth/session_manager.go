package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/models"
)

// Session represents a user session with credentials
type Session struct {
	Provider     string                 `json:"provider"`
	Email        string                 `json:"email"`
	Credentials  map[string]interface{} `json:"credentials"`
	ExpiresAt    time.Time              `json:"expires_at"`
	RefreshToken string                 `json:"refresh_token"`
}

// SessionManager manages stored credentials per provider
type SessionManager struct {
	db *database.DB
}

// NewSessionManager creates a new SessionManager instance
func NewSessionManager(db *database.DB) *SessionManager {
	return &SessionManager{
		db: db,
	}
}

// LoadSession loads a session from the database
func (sm *SessionManager) LoadSession(provider, email string) (*Session, error) {
	var acc models.Account
	err := sm.db.QueryRow(`
		SELECT id, email, password, provider, status, credits_used, credits_total,
		       token, cookie, last_used_at, last_synced_at, error_message, created_at, updated_at
		FROM accounts WHERE provider = ? AND email = ?
	`, provider, email).Scan(
		&acc.ID, &acc.Email, &acc.Password, &acc.Provider, &acc.Status,
		&acc.CreditsUsed, &acc.CreditsTotal, &acc.Token, &acc.Cookie,
		&acc.LastUsedAt, &acc.LastSyncedAt, &acc.ErrorMessage,
		&acc.CreatedAt, &acc.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// Parse credentials from token field (stored as JSON)
	credentials := make(map[string]interface{})
	if acc.Token != "" {
		if err := json.Unmarshal([]byte(acc.Token), &credentials); err != nil {
			// If token is not JSON, store it as a simple string
			credentials["token"] = acc.Token
		}
	}

	// Add cookie to credentials if present
	if acc.Cookie != "" {
		credentials["cookie"] = acc.Cookie
	}

	// Calculate expiry time (default: 24 hours from last sync)
	expiresAt := acc.UpdatedAt.Add(24 * time.Hour)
	if acc.LastSyncedAt != nil {
		expiresAt = acc.LastSyncedAt.Add(24 * time.Hour)
	}

	// Extract refresh token if present
	refreshToken := ""
	if rt, ok := credentials["refresh_token"].(string); ok {
		refreshToken = rt
	}

	return &Session{
		Provider:     acc.Provider,
		Email:        acc.Email,
		Credentials:  credentials,
		ExpiresAt:    expiresAt,
		RefreshToken: refreshToken,
	}, nil
}

// CheckExpiry checks if a session has expired
func (sm *SessionManager) CheckExpiry(session *Session) bool {
	return time.Now().After(session.ExpiresAt)
}

// RefreshToken refreshes the session token if expired
func (sm *SessionManager) RefreshToken(provider string, session *Session) error {
	switch provider {
	case models.ProviderKiro:
		return sm.refreshKiroToken(session)
	case models.ProviderCodeBuddy:
		return sm.refreshCodeBuddyToken(session)
	default:
		return fmt.Errorf("refresh not supported for provider: %s", provider)
	}
}

// refreshKiroToken refreshes Kiro token using refresh_token endpoint
func (sm *SessionManager) refreshKiroToken(session *Session) error {
	if session.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	// TODO: Implement actual refresh logic
	// For now, return error indicating manual re-login is needed
	return fmt.Errorf("kiro token refresh not yet implemented - manual re-login required")
}

// refreshCodeBuddyToken refreshes CodeBuddy session using saved cookies
func (sm *SessionManager) refreshCodeBuddyToken(session *Session) error {
	// CodeBuddy uses cookies which don't typically expire
	// Just validate that cookies are still present
	if cookie, ok := session.Credentials["cookie"].(string); !ok || cookie == "" {
		return fmt.Errorf("no cookie available for codebuddy session")
	}

	// TODO: Implement actual validation logic
	// For now, assume cookies are still valid
	return nil
}

// SaveSession saves a session to the database
func (sm *SessionManager) SaveSession(session *Session) error {
	// Serialize credentials to JSON
	credentialsJSON, err := json.Marshal(session.Credentials)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Extract cookie from credentials
	cookie := ""
	if c, ok := session.Credentials["cookie"].(string); ok {
		cookie = c
	}

	now := time.Now()

	// Update account in database
	_, err = sm.db.Exec(`
		UPDATE accounts 
		SET token = ?, cookie = ?, last_synced_at = ?, updated_at = ?
		WHERE provider = ? AND email = ?
	`, string(credentialsJSON), cookie, now, now, session.Provider, session.Email)

	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}
