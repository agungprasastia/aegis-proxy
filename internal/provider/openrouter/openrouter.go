package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
	"github.com/aegis-proxy/aegis/internal/provider/apikey"
)

const openRouterBaseURL = "https://openrouter.ai/api"

type OpenRouterProvider struct {
	engine *apikey.APIKeyProvider
}

type tokenData struct {
	APIKey string `json:"api_key"`
}

func init() {
	provider.ProviderRegistry["openrouter"] = NewOpenRouterProvider()
}

func NewOpenRouterProvider() *OpenRouterProvider {
	return &OpenRouterProvider{
		engine: apikey.NewAPIKeyProvider("OpenRouter", openRouterBaseURL, "Authorization"),
	}
}

func NewOpenRouterProviderWithBaseURL(baseURL string) *OpenRouterProvider {
	return &OpenRouterProvider{
		engine: apikey.NewAPIKeyProvider("OpenRouter", baseURL, "Authorization"),
	}
}

func (p *OpenRouterProvider) Name() string {
	return "OpenRouter"
}

func (p *OpenRouterProvider) Tier() string {
	return "openrouter"
}

func (p *OpenRouterProvider) SupportsModel(modelID string) bool {
	return strings.TrimSpace(modelID) != ""
}

func (p *OpenRouterProvider) MapModel(modelID string) string {
	return modelID
}

func (p *OpenRouterProvider) SendChatCompletion(ctx context.Context, account *models.Account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	apiKey, err := extractAPIKey(account)
	if err != nil {
		return nil, err
	}

	resp, err := p.engine.SendRequest(ctx, apiKey, toNormalized(req))
	if err != nil {
		return nil, normalizeError(err)
	}

	return toChatResponse(req.Model, resp), nil
}

func (p *OpenRouterProvider) SendChatCompletionStream(ctx context.Context, account *models.Account, req *provider.ChatRequest, writer http.ResponseWriter) error {
	return fmt.Errorf("OpenRouter streaming is not supported by API-key provider engine")
}

func (p *OpenRouterProvider) ValidateAccount(ctx context.Context, account *models.Account) error {
	apiKey, err := extractAPIKey(account)
	if err != nil {
		return err
	}
	if err := p.engine.TestConnectivity(ctx, apiKey); err != nil {
		return normalizeError(err)
	}
	return nil
}

func (p *OpenRouterProvider) GetCredits(ctx context.Context, account *models.Account) (used float64, total float64, err error) {
	return 0, 0, nil
}

func extractAPIKey(account *models.Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	if strings.TrimSpace(account.Token) == "" {
		return "", fmt.Errorf("account token is empty")
	}

	var data tokenData
	if err := json.Unmarshal([]byte(account.Token), &data); err == nil && strings.TrimSpace(data.APIKey) != "" {
		return data.APIKey, nil
	}
	return account.Token, nil
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "status 401") || strings.Contains(msg, "status 403") {
		return fmt.Errorf("authentication failed: invalid or expired api_key")
	}
	if strings.Contains(msg, "status 429") {
		return fmt.Errorf("rate limit exceeded")
	}
	if strings.Contains(msg, "status 5") {
		return fmt.Errorf("server error: %w", err)
	}
	return err
}

func toNormalized(req *provider.ChatRequest) *models.NormalizedRequest {
	if req == nil {
		return nil
	}
	messages := make([]models.NormalizedMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = models.NormalizedMessage{Role: msg.Role, Content: msg.Content}
	}
	return &models.NormalizedRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}
}

func toChatResponse(model string, resp *models.NormalizedResponse) *provider.ChatResponse {
	finishReason := ""
	if resp.FinishReason != "" {
		finishReason = string(resp.FinishReason)
	}
	return &provider.ChatResponse{
		ID:      "openrouter",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []provider.Choice{{
			Index:        0,
			Message:      provider.ChatMessage{Role: "assistant", Content: resp.Content},
			FinishReason: finishReason,
		}},
		Usage: provider.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}
