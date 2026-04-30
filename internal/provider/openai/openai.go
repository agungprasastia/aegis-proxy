package openai

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
	providerName = "openai"
	defaultBaseURL = "https://api.openai.com"
)

type OpenAIProvider struct {
	engine *apikey.APIKeyProvider
}

func init() {
	provider.ProviderRegistry[providerName] = NewOpenAIProvider()
}

func NewOpenAIProvider(baseURL ...string) *OpenAIProvider {
	url := defaultBaseURL
	if len(baseURL) > 0 && baseURL[0] != "" {
		url = baseURL[0]
	}

	return &OpenAIProvider{
		engine: apikey.NewAPIKeyProvider(providerName, url, "Authorization"),
	}
}

func (p *OpenAIProvider) Name() string {
	return providerName
}

func (p *OpenAIProvider) Tier() string {
	return "apikey"
}

func (p *OpenAIProvider) SendChatCompletion(ctx context.Context, account *models.Account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	apiKey, err := apiKey(account)
	if err != nil {
		return nil, err
	}

	resp, err := p.engine.SendRequest(ctx, apiKey, toNormalized(req))
	if err != nil {
		return nil, normalizeError(err)
	}

	return fromNormalized(req.Model, resp), nil
}

func (p *OpenAIProvider) SendChatCompletionStream(ctx context.Context, account *models.Account, req *provider.ChatRequest, writer http.ResponseWriter) error {
	return fmt.Errorf("streaming chat completions not supported by %s provider", providerName)
}

func (p *OpenAIProvider) ValidateAccount(ctx context.Context, account *models.Account) error {
	apiKey, err := apiKey(account)
	if err != nil {
		return err
	}
	if err := p.engine.TestConnectivity(ctx, apiKey); err != nil {
		return normalizeError(err)
	}
	return nil
}

func (p *OpenAIProvider) GetCredits(ctx context.Context, account *models.Account) (used float64, total float64, err error) {
	return 0, 0, nil
}

func (p *OpenAIProvider) SupportsModel(modelID string) bool {
	return modelID != ""
}

func (p *OpenAIProvider) MapModel(modelID string) string {
	return modelID
}

func (p *OpenAIProvider) ListModels(ctx context.Context, account *models.Account) ([]string, error) {
	apiKey, err := apiKey(account)
	if err != nil {
		return nil, err
	}
	models, err := p.engine.ListModels(ctx, apiKey)
	if err != nil {
		return nil, normalizeError(err)
	}
	return models, nil
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
	for i, message := range req.Messages {
		messages[i] = models.NormalizedMessage{Role: message.Role, Content: message.Content}
	}

	return &models.NormalizedRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}
}

func fromNormalized(model string, resp *models.NormalizedResponse) *provider.ChatResponse {
	if resp == nil {
		return nil
	}

	return &provider.ChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []provider.Choice{{
			Index:        0,
			Message:      provider.ChatMessage{Role: "assistant", Content: resp.Content},
			FinishReason: string(resp.FinishReason),
		}},
		Usage: provider.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "status 401") || strings.Contains(msg, "status 403") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "invalid api key"):
		return fmt.Errorf("authentication failed: invalid or expired api_key")
	case strings.Contains(msg, "status 429") || strings.Contains(msg, "rate limit"):
		return fmt.Errorf("rate limit exceeded")
	default:
		return err
	}
}
