package gemini

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

const (
	providerName   = "gemini"
	defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
)

type GeminiProvider struct {
	engine *apikey.APIKeyProvider
}

type tokenData struct {
	APIKey string `json:"api_key"`
}

func init() {
	provider.ProviderRegistry[providerName] = NewGeminiProvider()
}

func NewGeminiProvider(baseURL ...string) *GeminiProvider {
	url := defaultBaseURL
	if len(baseURL) > 0 && baseURL[0] != "" {
		url = baseURL[0]
	}

	engine := apikey.NewAPIKeyProvider(providerName, url, "Authorization")
	engine.SetAPIPrefix("")
	return &GeminiProvider{engine: engine}
}

func (p *GeminiProvider) Name() string {
	return providerName
}

func (p *GeminiProvider) Tier() string {
	return "apikey"
}

func (p *GeminiProvider) SupportsModel(modelID string) bool {
	return strings.TrimSpace(modelID) != ""
}

func (p *GeminiProvider) MapModel(modelID string) string {
	return modelID
}

func (p *GeminiProvider) SendChatCompletion(ctx context.Context, account *models.Account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	if hasUnsupportedMultimodal(req) {
		return nil, fmt.Errorf("gemini provider does not support multimodal fields in v1")
	}

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

func (p *GeminiProvider) SendChatCompletionStream(ctx context.Context, account *models.Account, req *provider.ChatRequest, writer http.ResponseWriter) error {
	return fmt.Errorf("streaming chat completions not supported by %s provider", providerName)
}

func (p *GeminiProvider) ValidateAccount(ctx context.Context, account *models.Account) error {
	apiKey, err := extractAPIKey(account)
	if err != nil {
		return err
	}
	if err := p.engine.TestConnectivity(ctx, apiKey); err != nil {
		return normalizeError(err)
	}
	return nil
}

func (p *GeminiProvider) GetCredits(ctx context.Context, account *models.Account) (used float64, total float64, err error) {
	return 0, 0, nil
}

func (p *GeminiProvider) ListModels(ctx context.Context, account *models.Account) ([]string, error) {
	apiKey, err := extractAPIKey(account)
	if err != nil {
		return nil, err
	}
	models, err := p.engine.ListModels(ctx, apiKey)
	if err != nil {
		return nil, normalizeError(err)
	}
	return models, nil
}

func extractAPIKey(account *models.Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	if strings.TrimSpace(account.Token) == "" {
		return "", fmt.Errorf("api key is empty")
	}

	var data tokenData
	if err := json.Unmarshal([]byte(account.Token), &data); err == nil && strings.TrimSpace(data.APIKey) != "" {
		return data.APIKey, nil
	}
	return account.Token, nil
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

func toChatResponse(model string, resp *models.NormalizedResponse) *provider.ChatResponse {
	finishReason := ""
	if resp.FinishReason != "" {
		finishReason = string(resp.FinishReason)
	}
	return &provider.ChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
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

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "authentication failed") || strings.Contains(msg, "status 401") || strings.Contains(msg, "status 403") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "api key not valid"):
		return fmt.Errorf("authentication failed: invalid or expired api_key")
	case strings.Contains(msg, "status 429") || strings.Contains(msg, "rate limit"):
		return fmt.Errorf("rate limit exceeded")
	default:
		return err
	}
}

func hasUnsupportedMultimodal(req *provider.ChatRequest) bool {
	if req == nil {
		return false
	}
	if len(req.Tools) > 0 {
		return true
	}
	for _, message := range req.Messages {
		content := strings.TrimSpace(message.Content)
		if strings.HasPrefix(content, "[") || strings.Contains(content, "image_url") || strings.Contains(content, "input_audio") || strings.Contains(content, "file_data") {
			return true
		}
	}
	return false
}
