package xai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
	"github.com/aegis-proxy/aegis/internal/provider/apikey"
)

const (
	providerName   = "xai"
	defaultBaseURL = "https://api.x.ai"
)

type Provider struct{ engine *apikey.APIKeyProvider }

func init() { provider.ProviderRegistry[providerName] = NewProvider() }
func NewProvider(baseURL ...string) *Provider {
	url := defaultBaseURL
	if len(baseURL) > 0 && baseURL[0] != "" {
		url = baseURL[0]
	}
	return &Provider{engine: apikey.NewAPIKeyProvider(providerName, url, "Authorization")}
}
func (p *Provider) Name() string { return providerName }
func (p *Provider) Tier() string { return "apikey" }
func (p *Provider) SendChatCompletion(ctx context.Context, account *models.Account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	key, err := apiKey(account)
	if err != nil {
		return nil, err
	}
	resp, err := p.engine.SendRequest(ctx, key, toNormalized(req))
	if err != nil {
		return nil, normalizeError(err)
	}
	return fromNormalized(req.Model, resp), nil
}
func (p *Provider) SendChatCompletionStream(ctx context.Context, account *models.Account, req *provider.ChatRequest, writer http.ResponseWriter) error {
	return fmt.Errorf("streaming chat completions not supported by %s provider", providerName)
}
func (p *Provider) ValidateAccount(ctx context.Context, account *models.Account) error {
	key, err := apiKey(account)
	if err != nil {
		return err
	}
	return normalizeError(p.engine.TestConnectivity(ctx, key))
}
func (p *Provider) GetCredits(ctx context.Context, account *models.Account) (float64, float64, error) {
	return 0, 0, nil
}
func (p *Provider) SupportsModel(modelID string) bool {
	info, ok := models.GetModelInfo(modelID)
	return ok && info.Provider == models.ProviderXAI
}
func (p *Provider) MapModel(modelID string) string { return modelID }
func (p *Provider) ListModels(ctx context.Context, account *models.Account) ([]string, error) {
	key, err := apiKey(account)
	if err != nil {
		return nil, err
	}
	m, err := p.engine.ListModels(ctx, key)
	return m, normalizeError(err)
}
func apiKey(account *models.Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	if account.Token != "" {
		return account.Token, nil
	}
	if account.Password != "" {
		return account.Password, nil
	}
	return "", fmt.Errorf("api key is empty")
}
func toNormalized(req *provider.ChatRequest) *models.NormalizedRequest {
	if req == nil {
		return nil
	}
	messages := make([]models.NormalizedMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = models.NormalizedMessage{Role: m.Role, Content: m.Content}
	}
	return &models.NormalizedRequest{Model: req.Model, Messages: messages, MaxTokens: req.MaxTokens, Temperature: req.Temperature, Stream: req.Stream}
}
func fromNormalized(model string, resp *models.NormalizedResponse) *provider.ChatResponse {
	if resp == nil {
		return nil
	}
	return &provider.ChatResponse{ID: fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()), Object: "chat.completion", Created: time.Now().Unix(), Model: model, Choices: []provider.Choice{{Index: 0, Message: provider.ChatMessage{Role: "assistant", Content: resp.Content}, FinishReason: string(resp.FinishReason)}}, Usage: provider.Usage{PromptTokens: resp.Usage.PromptTokens, CompletionTokens: resp.Usage.CompletionTokens, TotalTokens: resp.Usage.TotalTokens}}
}
func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "status 401") || strings.Contains(msg, "status 403") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "authentication failed") || strings.Contains(msg, "invalid api key"):
		return fmt.Errorf("xai authentication failed: invalid or expired api_key")
	case strings.Contains(msg, "status 429") || strings.Contains(msg, "rate limit"):
		return fmt.Errorf("xai rate limit exceeded")
	default:
		return err
	}
}
