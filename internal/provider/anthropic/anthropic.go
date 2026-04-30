package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
	"github.com/aegis-proxy/aegis/internal/provider/apikey"
)

const (
	defaultBaseURL         = "https://api.anthropic.com"
	anthropicVersionHeader = "2023-06-01"
)

type AnthropicProvider struct {
	engine *apikey.APIKeyProvider
	client *http.Client
}

type tokenData struct {
	APIKey string `json:"api_key"`
}

type messageRequest struct {
	Model       string          `json:"model"`
	Messages    []message       `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Tools       []anthropicTool `json:"tools,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type messageResponse struct {
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      usage          `json:"usage"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func init() {
	provider.ProviderRegistry["anthropic"] = NewAnthropicProvider(defaultBaseURL)
}

func NewAnthropicProvider(baseURL string) *AnthropicProvider {
	return &AnthropicProvider{
		engine: apikey.NewAPIKeyProvider("anthropic", baseURL, "x-api-key"),
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *AnthropicProvider) Name() string {
	return "Anthropic"
}

func (p *AnthropicProvider) Tier() string {
	return "anthropic"
}

func (p *AnthropicProvider) SendChatCompletion(ctx context.Context, account *models.Account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	apiKey, err := extractAPIKey(account)
	if err != nil {
		return nil, err
	}

	normalized := chatRequestToNormalized(req)
	resp, err := p.sendMessages(ctx, apiKey, normalized)
	if err != nil {
		return nil, err
	}

	return normalizedToChatResponse(normalized.Model, resp), nil
}

func (p *AnthropicProvider) SendChatCompletionStream(ctx context.Context, account *models.Account, req *provider.ChatRequest, writer http.ResponseWriter) error {
	return fmt.Errorf("anthropic streaming not supported")
}

func (p *AnthropicProvider) ValidateAccount(ctx context.Context, account *models.Account) error {
	apiKey, err := extractAPIKey(account)
	if err != nil {
		return err
	}
	if err := p.engine.TestConnectivity(ctx, apiKey); err != nil {
		return normalizeError(err)
	}
	return nil
}

func (p *AnthropicProvider) GetCredits(ctx context.Context, account *models.Account) (used float64, total float64, err error) {
	return 0, 0, nil
}

func (p *AnthropicProvider) SupportsModel(modelID string) bool {
	return modelID != ""
}

func (p *AnthropicProvider) MapModel(modelID string) string {
	return modelID
}

func (p *AnthropicProvider) ListModels(ctx context.Context, account *models.Account) ([]string, error) {
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

func (p *AnthropicProvider) sendMessages(ctx context.Context, apiKey string, req *models.NormalizedRequest) (*models.NormalizedResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	body, err := json.Marshal(normalizedToAnthropic(req))
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.engine.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersionHeader)

	resp, err := p.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, normalizeHTTPError(resp.StatusCode, body)
	}

	var payload messageResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return anthropicToNormalized(&payload), nil
}

func (p *AnthropicProvider) httpClient() *http.Client {
	if p.client != nil {
		return p.client
	}
	return http.DefaultClient
}

func extractAPIKey(account *models.Account) (string, error) {
	if account == nil || account.Token == "" {
		return "", fmt.Errorf("api key missing")
	}

	var token tokenData
	if err := json.Unmarshal([]byte(account.Token), &token); err == nil && token.APIKey != "" {
		return token.APIKey, nil
	}

	return account.Token, nil
}

func chatRequestToNormalized(req *provider.ChatRequest) *models.NormalizedRequest {
	if req == nil {
		return nil
	}

	messages := make([]models.NormalizedMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = models.NormalizedMessage{Role: msg.Role, Content: msg.Content}
	}

	tools := make([]models.NormalizedToolCall, len(req.Tools))
	for i, tool := range req.Tools {
		args, _ := json.Marshal(tool.Function.Parameters)
		tools[i] = models.NormalizedToolCall{Name: tool.Function.Name, Arguments: string(args)}
	}

	return &models.NormalizedRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Tools:       tools,
		Stream:      req.Stream,
	}
}

func normalizedToAnthropic(req *models.NormalizedRequest) *messageRequest {
	messages := make([]message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = message{Role: msg.Role, Content: msg.Content}
	}

	tools := make([]anthropicTool, len(req.Tools))
	for i, tool := range req.Tools {
		tools[i] = anthropicTool{Name: tool.Name, InputSchema: json.RawMessage(tool.Arguments)}
	}

	return &messageRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Tools:       tools,
		Stream:      req.Stream,
	}
}

func anthropicToNormalized(resp *messageResponse) *models.NormalizedResponse {
	content := ""
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &models.NormalizedResponse{
		Content:      content,
		FinishReason: mapFinishReason(resp.StopReason),
		Usage: models.NormalizedUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

func normalizedToChatResponse(model string, resp *models.NormalizedResponse) *provider.ChatResponse {
	return &provider.ChatResponse{
		ID:      "",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []provider.Choice{{
			Index: 0,
			Message: provider.ChatMessage{
				Role:    "assistant",
				Content: resp.Content,
			},
			FinishReason: string(resp.FinishReason),
		}},
		Usage: provider.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}

func mapFinishReason(reason string) models.FinishReason {
	switch reason {
	case "end_turn", "stop_sequence", "":
		return models.FinishReasonStop
	case "max_tokens":
		return models.FinishReasonLength
	case "tool_use":
		return models.FinishReasonToolCalls
	default:
		return models.FinishReasonError
	}
}

func normalizeHTTPError(status int, body []byte) error {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return fmt.Errorf("authentication failed: invalid or expired api_key")
	}
	if status == http.StatusTooManyRequests {
		return fmt.Errorf("rate limit exceeded")
	}
	if status >= 500 {
		return fmt.Errorf("server error: status %d", status)
	}
	return fmt.Errorf("anthropic request failed: status %d: %s", status, string(body))
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("authentication failed: invalid or expired api_key")
}
