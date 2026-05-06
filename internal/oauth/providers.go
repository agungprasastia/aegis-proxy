package oauth

import (
	"context"
	"errors"
)

const (
	ProviderClaudeCode   = "claude_code"
	ProviderCodex        = "codex"
	ProviderGitHubCopilot = "github_copilot"
	ProviderCursor       = "cursor"
)

type ProviderSpec struct {
	Provider    string
	AuthURL     string
	TokenURL    string
	Scopes      []string
	APIBaseURL  string
	TokenHeader string
}

var providerSpecs = map[string]ProviderSpec{
	ProviderClaudeCode: {
		Provider: ProviderClaudeCode,
		AuthURL: "https://claude.ai/oauth/authorize",
		TokenURL: "https://claude.ai/oauth/token",
		Scopes: []string{"openid", "profile", "offline_access"},
		APIBaseURL: "https://api.anthropic.com/v1",
		TokenHeader: "Authorization",
	},
	ProviderCodex: {
		Provider: ProviderCodex,
		AuthURL: "https://auth.openai.com/oauth/authorize",
		TokenURL: "https://auth.openai.com/oauth/token",
		Scopes: []string{"openid", "profile", "email", "offline_access"},
		APIBaseURL: "https://api.openai.com/v1",
		TokenHeader: "Authorization",
	},
	ProviderGitHubCopilot: {
		Provider: ProviderGitHubCopilot,
		AuthURL: "https://github.com/login/oauth/authorize",
		TokenURL: "https://github.com/login/oauth/access_token",
		Scopes: []string{"read:user", "user:email", "copilot"},
		APIBaseURL: "https://api.githubcopilot.com",
		TokenHeader: "Authorization",
	},
	ProviderCursor: {
		Provider: ProviderCursor,
		AuthURL: "https://cursor.com/oauth/authorize",
		TokenURL: "https://cursor.com/oauth/token",
		Scopes: []string{"openid", "profile", "offline_access"},
		APIBaseURL: "https://api.cursor.com",
		TokenHeader: "Authorization",
	},
}

func ProviderConfigFor(provider, clientID, redirectURI string) (ProviderConfig, error) {
	spec, ok := providerSpecs[provider]
	if !ok {
		return ProviderConfig{}, errors.New("unknown oauth provider")
	}
	return ProviderConfig{Provider: spec.Provider, ClientID: clientID, RedirectURI: redirectURI, AuthURL: spec.AuthURL, TokenURL: spec.TokenURL, Scopes: append([]string(nil), spec.Scopes...)}, nil
}

func ProviderSpecFor(provider string) (ProviderSpec, error) {
	spec, ok := providerSpecs[provider]
	if !ok {
		return ProviderSpec{}, errors.New("unknown oauth provider")
	}
	return spec, nil
}

func (e *Engine) StartProviderFlow(provider, clientID, redirectURI string) (Flow, error) {
	cfg, err := ProviderConfigFor(provider, clientID, redirectURI)
	if err != nil {
		return Flow{}, err
	}
	return e.StartFlow(cfg, 0)
}

func (e *Engine) RefreshProviderToken(ctx context.Context, provider, clientID, redirectURI, refreshToken string) (*Token, error) {
	cfg, err := ProviderConfigFor(provider, clientID, redirectURI)
	if err != nil {
		return nil, err
	}
	return e.Refresh(ctx, cfg, refreshToken)
}
