package oauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aegis-proxy/aegis/internal/database"
)

var ErrFlowNotFound = errors.New("oauth flow not found")

type ProviderConfig struct {
	Provider    string
	ClientID    string
	RedirectURI string
	AuthURL     string
	TokenURL    string
	Scopes      []string
}

type Flow struct {
	Provider      string
	State         string
	Verifier      string
	Challenge     string
	AuthorizeURL  string
	ExpiresAt     time.Time
}

type Token struct {
	Provider     string     `json:"provider"`
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Engine struct {
	db     *database.DB
	client *http.Client
	mu     sync.Mutex
	flows  map[string]Flow
}

func NewEngine(db *database.DB, client *http.Client) *Engine {
	if client == nil {
		client = http.DefaultClient
	}
	return &Engine{db: db, client: client, flows: map[string]Flow{}}
}

func (e *Engine) StartFlow(cfg ProviderConfig, ttl time.Duration) (Flow, error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	if cfg.Provider == "" || cfg.ClientID == "" || cfg.RedirectURI == "" || cfg.AuthURL == "" || cfg.TokenURL == "" {
		return Flow{}, errors.New("provider, client ID, redirect URI, auth URL, and token URL are required")
	}
	state, err := randomURLString(32)
	if err != nil {
		return Flow{}, err
	}
	verifier, err := randomURLString(64)
	if err != nil {
		return Flow{}, err
	}
	challenge := pkceChallenge(verifier)
	authURL, err := buildAuthorizeURL(cfg, state, challenge)
	if err != nil {
		return Flow{}, err
	}
	flow := Flow{Provider: cfg.Provider, State: state, Verifier: verifier, Challenge: challenge, AuthorizeURL: authURL, ExpiresAt: time.Now().UTC().Add(ttl)}
	e.mu.Lock()
	e.flows[state] = flow
	e.mu.Unlock()
	return flow, nil
}

func (e *Engine) CompleteCallback(ctx context.Context, cfg ProviderConfig, state, code string) (*Token, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("oauth callback missing code")
	}
	flow, err := e.takeFlow(state)
	if err != nil {
		return nil, err
	}
	if time.Now().UTC().After(flow.ExpiresAt) {
		return nil, errors.New("oauth flow expired")
	}
	if flow.Provider != cfg.Provider {
		return nil, errors.New("oauth provider mismatch")
	}
	token, err := e.exchange(ctx, cfg, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {cfg.ClientID},
		"code":          {code},
		"redirect_uri":  {cfg.RedirectURI},
		"code_verifier": {flow.Verifier},
	})
	if err != nil {
		return nil, err
	}
	if err := e.StoreToken(ctx, token); err != nil {
		return nil, err
	}
	return token, nil
}

func (e *Engine) Refresh(ctx context.Context, cfg ProviderConfig, refreshToken string) (*Token, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, errors.New("refresh token is required")
	}
	token, err := e.exchange(ctx, cfg, url.Values{"grant_type": {"refresh_token"}, "client_id": {cfg.ClientID}, "refresh_token": {refreshToken}})
	if err != nil {
		return nil, err
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	if err := e.StoreToken(ctx, token); err != nil {
		return nil, err
	}
	return token, nil
}

func (e *Engine) Disconnect(ctx context.Context, provider string) error {
	if e == nil || e.db == nil {
		return errors.New("oauth store is not configured")
	}
	_, err := e.db.SQL().ExecContext(ctx, `DELETE FROM oauth_tokens WHERE provider = ?`, provider)
	return err
}

func (e *Engine) StoreToken(ctx context.Context, token *Token) error {
	if e == nil || e.db == nil {
		return errors.New("oauth store is not configured")
	}
	if token == nil || token.Provider == "" || token.AccessToken == "" {
		return errors.New("provider and access token are required")
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}
	_, err := e.db.SQL().ExecContext(ctx, `
		INSERT INTO oauth_tokens (provider, access_token, refresh_token, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, token.Provider, token.AccessToken, token.RefreshToken, token.ExpiresAt, token.CreatedAt)
	return err
}

func (e *Engine) LatestToken(ctx context.Context, provider string) (*Token, error) {
	if e == nil || e.db == nil {
		return nil, errors.New("oauth store is not configured")
	}
	var token Token
	var expires sql.NullTime
	err := e.db.SQL().QueryRowContext(ctx, `
		SELECT provider, access_token, refresh_token, expires_at, created_at
		FROM oauth_tokens WHERE provider = ? ORDER BY created_at DESC, id DESC LIMIT 1
	`, provider).Scan(&token.Provider, &token.AccessToken, &token.RefreshToken, &expires, &token.CreatedAt)
	if err != nil {
		return nil, err
	}
	if expires.Valid {
		token.ExpiresAt = &expires.Time
	}
	return &token, nil
}

func (e *Engine) takeFlow(state string) (Flow, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	flow, ok := e.flows[state]
	if !ok {
		return Flow{}, ErrFlowNotFound
	}
	delete(e.flows, state)
	return flow, nil
}

func (e *Engine) exchange(ctx context.Context, cfg ProviderConfig, form url.Values) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oauth token exchange failed: status %d", resp.StatusCode)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.AccessToken == "" {
		return nil, errors.New("oauth token response missing access token")
	}
	token := &Token{Provider: cfg.Provider, AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, CreatedAt: time.Now().UTC()}
	if payload.ExpiresIn > 0 {
		expires := token.CreatedAt.Add(time.Duration(payload.ExpiresIn) * time.Second)
		token.ExpiresAt = &expires
	}
	return token, nil
}

func buildAuthorizeURL(cfg ProviderConfig, state, challenge string) (string, error) {
	u, err := url.Parse(cfg.AuthURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURI)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if len(cfg.Scopes) > 0 {
		q.Set("scope", strings.Join(cfg.Scopes, " "))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomURLString(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
