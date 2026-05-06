package translate

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aegis-proxy/aegis/internal/models"
)

var ErrUnsupportedMultimodal = errors.New("unsupported multimodal content in translation v1")

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type OpenAIMessage struct {
	Role      string             `json:"role"`
	Content   string             `json:"content"`
	ToolCalls []OpenAIToolCall   `json:"tool_calls,omitempty"`
	ToolCallID string             `json:"tool_call_id,omitempty"`
}

type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

type OpenAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type OpenAIToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function OpenAICallFunc `json:"function"`
}

type OpenAICallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message,omitempty"`
	Delta        *OpenAIDelta  `json:"delta,omitempty"`
	FinishReason string        `json:"finish_reason,omitempty"`
}

type OpenAIDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type AnthropicRequest struct {
	Model       string             `json:"model"`
	System      string             `json:"system,omitempty"`
	Messages    []AnthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	Tools       []AnthropicTool    `json:"tools,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type AnthropicMessage struct {
	Role    string             `json:"role"`
	Content []AnthropicContent `json:"content"`
}

type AnthropicContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type AnthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []AnthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason,omitempty"`
	StopSequence string             `json:"stop_sequence,omitempty"`
	Usage        AnthropicUsage     `json:"usage"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type AnthropicStreamEvent struct {
	Type         string             `json:"type"`
	Index        int                `json:"index,omitempty"`
	Delta        *AnthropicDelta    `json:"delta,omitempty"`
	Message      *AnthropicResponse `json:"message,omitempty"`
	ContentBlock *AnthropicContent  `json:"content_block,omitempty"`
	Usage        *AnthropicUsage    `json:"usage,omitempty"`
}

type AnthropicDelta struct {
	Type       string `json:"type,omitempty"`
	Text       string `json:"text,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
}

func OpenAIToNormalized(req OpenAIRequest) (*models.NormalizedRequest, error) {
	messages := make([]models.NormalizedMessage, 0, len(req.Messages))
	for _, message := range req.Messages {
		messages = append(messages, models.NormalizedMessage{
			Role:      message.Role,
			Content:   message.Content,
			ToolCalls: openAIToolCallsToNormalized(message.ToolCalls),
		})
	}
	return &models.NormalizedRequest{Model: req.Model, Messages: messages, MaxTokens: req.MaxTokens, Temperature: req.Temperature, Tools: openAIToolsToNormalized(req.Tools), Stream: req.Stream}, nil
}

func DecodeOpenAIRequest(data []byte) (*models.NormalizedRequest, error) {
	if err := rejectContentArrays(data); err != nil {
		return nil, err
	}
	var req OpenAIRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return OpenAIToNormalized(req)
}

func AnthropicToNormalized(req AnthropicRequest) (*models.NormalizedRequest, error) {
	messages := make([]models.NormalizedMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		messages = append(messages, models.NormalizedMessage{Role: "system", Content: req.System})
	}
	for _, message := range req.Messages {
		content, calls, err := anthropicContentToNormalized(message.Content)
		if err != nil {
			return nil, err
		}
		messages = append(messages, models.NormalizedMessage{Role: message.Role, Content: content, ToolCalls: calls})
	}
	return &models.NormalizedRequest{Model: req.Model, Messages: messages, MaxTokens: req.MaxTokens, Temperature: req.Temperature, Tools: anthropicToolsToNormalized(req.Tools), Stream: req.Stream}, nil
}

func NormalizedToOpenAI(req *models.NormalizedRequest, resp *models.NormalizedResponse) OpenAIResponse {
	message := OpenAIMessage{Role: "assistant", Content: resp.Content, ToolCalls: normalizedToolCallsToOpenAI(resp.ToolCalls)}
	return OpenAIResponse{ID: "chatcmpl-aegis", Object: "chat.completion", Created: time.Now().Unix(), Model: req.Model, Choices: []OpenAIChoice{{Index: 0, Message: message, FinishReason: normalizedToOpenAIFinish(resp.FinishReason)}}, Usage: OpenAIUsage{PromptTokens: resp.Usage.PromptTokens, CompletionTokens: resp.Usage.CompletionTokens, TotalTokens: resp.Usage.TotalTokens}}
}

func NormalizedToAnthropic(req *models.NormalizedRequest, resp *models.NormalizedResponse) AnthropicResponse {
	content := []AnthropicContent{{Type: "text", Text: resp.Content}}
	for _, call := range resp.ToolCalls {
		content = append(content, normalizedToolCallToAnthropicContent(call))
	}
	return AnthropicResponse{ID: "msg_aegis", Type: "message", Role: "assistant", Content: content, Model: req.Model, StopReason: normalizedToAnthropicFinish(resp.FinishReason), Usage: AnthropicUsage{InputTokens: resp.Usage.PromptTokens, OutputTokens: resp.Usage.CompletionTokens}}
}

func OpenAIStreamToNormalized(chunk OpenAIResponse) models.NormalizedResponse {
	resp := models.NormalizedResponse{}
	if len(chunk.Choices) == 0 {
		return resp
	}
	choice := chunk.Choices[0]
	if choice.Delta != nil {
		resp.Content = choice.Delta.Content
		resp.ToolCalls = openAIToolCallsToNormalized(choice.Delta.ToolCalls)
	}
	resp.FinishReason = openAIFinishToNormalized(choice.FinishReason)
	return resp
}

func NormalizedStreamToAnthropic(resp models.NormalizedResponse) []AnthropicStreamEvent {
	events := make([]AnthropicStreamEvent, 0, 2)
	if resp.Content != "" {
		events = append(events, AnthropicStreamEvent{Type: "content_block_delta", Index: 0, Delta: &AnthropicDelta{Type: "text_delta", Text: resp.Content}})
	}
	if resp.FinishReason != "" {
		events = append(events, AnthropicStreamEvent{Type: "message_delta", Delta: &AnthropicDelta{StopReason: normalizedToAnthropicFinish(resp.FinishReason)}})
		events = append(events, AnthropicStreamEvent{Type: "message_stop"})
	}
	return events
}

func AnthropicStreamToNormalized(event AnthropicStreamEvent) models.NormalizedResponse {
	resp := models.NormalizedResponse{}
	if event.Delta != nil {
		resp.Content = event.Delta.Text
		resp.FinishReason = anthropicFinishToNormalized(event.Delta.StopReason)
	}
	if event.Usage != nil {
		resp.Usage = models.NormalizedUsage{PromptTokens: event.Usage.InputTokens, CompletionTokens: event.Usage.OutputTokens, TotalTokens: event.Usage.InputTokens + event.Usage.OutputTokens}
	}
	return resp
}

func NormalizedStreamToOpenAI(req *models.NormalizedRequest, resp models.NormalizedResponse) OpenAIResponse {
	return OpenAIResponse{ID: "chatcmpl-aegis", Object: "chat.completion.chunk", Created: time.Now().Unix(), Model: req.Model, Choices: []OpenAIChoice{{Index: 0, Delta: &OpenAIDelta{Content: resp.Content, ToolCalls: normalizedToolCallsToOpenAI(resp.ToolCalls)}, FinishReason: normalizedToOpenAIFinish(resp.FinishReason)}}}
}

func openAIToolsToNormalized(tools []OpenAITool) []models.NormalizedToolCall {
	if len(tools) == 0 {
		return nil
	}
	out := make([]models.NormalizedToolCall, 0, len(tools))
	for _, tool := range tools {
		out = append(out, models.NormalizedToolCall{Name: tool.Function.Name, Arguments: string(tool.Function.Parameters)})
	}
	return out
}

func openAIToolCallsToNormalized(calls []OpenAIToolCall) []models.NormalizedToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]models.NormalizedToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, models.NormalizedToolCall{ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
	}
	return out
}

func normalizedToolCallsToOpenAI(calls []models.NormalizedToolCall) []OpenAIToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]OpenAIToolCall, 0, len(calls))
	for _, call := range calls {
		out = append(out, OpenAIToolCall{ID: call.ID, Type: "function", Function: OpenAICallFunc{Name: call.Name, Arguments: call.Arguments}})
	}
	return out
}

func anthropicToolsToNormalized(tools []AnthropicTool) []models.NormalizedToolCall {
	if len(tools) == 0 {
		return nil
	}
	out := make([]models.NormalizedToolCall, 0, len(tools))
	for _, tool := range tools {
		out = append(out, models.NormalizedToolCall{Name: tool.Name, Arguments: string(tool.InputSchema)})
	}
	return out
}

func anthropicContentToNormalized(blocks []AnthropicContent) (string, []models.NormalizedToolCall, error) {
	text := ""
	calls := []models.NormalizedToolCall{}
	for _, block := range blocks {
		switch block.Type {
		case "text":
			text += block.Text
		case "tool_use":
			calls = append(calls, models.NormalizedToolCall{ID: block.ID, Name: block.Name, Arguments: string(block.Input)})
		default:
			return "", nil, fmt.Errorf("%w: anthropic content type %q", ErrUnsupportedMultimodal, block.Type)
		}
	}
	if len(calls) == 0 {
		return text, nil, nil
	}
	return text, calls, nil
}

func normalizedToolCallToAnthropicContent(call models.NormalizedToolCall) AnthropicContent {
	input := json.RawMessage(call.Arguments)
	if !json.Valid(input) {
		input = json.RawMessage(`{}`)
	}
	return AnthropicContent{Type: "tool_use", ID: call.ID, Name: call.Name, Input: input}
}

func rejectContentArrays(data []byte) error {
	var raw struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, message := range raw.Messages {
		if len(message.Content) > 0 && message.Content[0] == '[' {
			return ErrUnsupportedMultimodal
		}
	}
	return nil
}

func openAIFinishToNormalized(reason string) models.FinishReason {
	switch reason {
	case "stop":
		return models.FinishReasonStop
	case "length":
		return models.FinishReasonLength
	case "tool_calls":
		return models.FinishReasonToolCalls
	case "content_filter":
		return models.FinishReasonError
	default:
		return ""
	}
}

func normalizedToOpenAIFinish(reason models.FinishReason) string {
	switch reason {
	case models.FinishReasonStop:
		return "stop"
	case models.FinishReasonLength:
		return "length"
	case models.FinishReasonToolCalls:
		return "tool_calls"
	case models.FinishReasonError:
		return "content_filter"
	default:
		return ""
	}
}

func anthropicFinishToNormalized(reason string) models.FinishReason {
	switch reason {
	case "end_turn", "stop_sequence":
		return models.FinishReasonStop
	case "max_tokens":
		return models.FinishReasonLength
	case "tool_use":
		return models.FinishReasonToolCalls
	default:
		return ""
	}
}

func normalizedToAnthropicFinish(reason models.FinishReason) string {
	switch reason {
	case models.FinishReasonStop:
		return "end_turn"
	case models.FinishReasonLength:
		return "max_tokens"
	case models.FinishReasonToolCalls:
		return "tool_use"
	case models.FinishReasonError:
		return "error"
	default:
		return ""
	}
}
