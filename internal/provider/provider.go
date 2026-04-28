package provider

import (
	"context"
	"net/http"

	"github.com/aegis-proxy/aegis/internal/models"
)

type Provider interface {
	Name() string
	Tier() string
	SendChatCompletion(ctx context.Context, account *models.Account, req *ChatRequest) (*ChatResponse, error)
	SendChatCompletionStream(ctx context.Context, account *models.Account, req *ChatRequest, writer http.ResponseWriter) error
	ValidateAccount(ctx context.Context, account *models.Account) error
	GetCredits(ctx context.Context, account *models.Account) (used float64, total float64, err error)
	SupportsModel(modelID string) bool
	MapModel(modelID string) string
}

type ChatRequest struct {
	Model           string        `json:"model"`
	Messages        []ChatMessage `json:"messages"`
	Stream          bool          `json:"stream"`
	MaxTokens       int           `json:"max_tokens,omitempty"`
	Temperature     float64       `json:"temperature,omitempty"`
	Tools           []Tool        `json:"tools,omitempty"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Tool struct {
	Type     string      `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []StreamDelta `json:"choices"`
}

type StreamDelta struct {
	Index        int               `json:"index"`
	Delta        ChatMessageDelta  `json:"delta"`
	FinishReason string            `json:"finish_reason,omitempty"`
}

type ChatMessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

var ProviderRegistry = make(map[string]Provider)
