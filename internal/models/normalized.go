package models

type NormalizedRequest struct {
	Model       string               `json:"model"`
	Messages    []NormalizedMessage  `json:"messages"`
	MaxTokens   int                  `json:"max_tokens,omitempty"`
	Temperature float64              `json:"temperature,omitempty"`
	Tools       []NormalizedToolCall `json:"tools,omitempty"`
	Stream      bool                 `json:"stream,omitempty"`
}

type NormalizedMessage struct {
	Role      string               `json:"role"`
	Content   string               `json:"content"`
	ToolCalls []NormalizedToolCall `json:"tool_calls,omitempty"`
}

type NormalizedResponse struct {
	Content      string               `json:"content"`
	FinishReason FinishReason         `json:"finish_reason"`
	Usage        NormalizedUsage      `json:"usage"`
	ToolCalls    []NormalizedToolCall `json:"tool_calls,omitempty"`
}

type NormalizedUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type NormalizedToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type FinishReason string

const (
	FinishReasonStop      FinishReason = "stop"
	FinishReasonLength    FinishReason = "length"
	FinishReasonToolCalls FinishReason = "tool_calls"
	FinishReasonError     FinishReason = "error"
)
