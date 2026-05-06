package translate

import (
	"errors"
	"testing"

	"github.com/aegis-proxy/aegis/internal/models"
)

func TestOpenAIToNormalizedWithTools(t *testing.T) {
	req := OpenAIRequest{
		Model: "gpt-4o",
		Messages: []OpenAIMessage{{Role: "user", Content: "hello", ToolCalls: []OpenAIToolCall{{ID: "call_1", Type: "function", Function: OpenAICallFunc{Name: "lookup", Arguments: `{"q":"x"}`}}}}},
		Tools: []OpenAITool{{Type: "function", Function: OpenAIFunction{Name: "lookup", Parameters: []byte(`{"type":"object"}`)}}},
		MaxTokens: 100,
		Stream: true,
	}

	normalized, err := OpenAIToNormalized(req)
	if err != nil {
		t.Fatalf("OpenAIToNormalized returned error: %v", err)
	}
	if normalized.Model != "gpt-4o" || !normalized.Stream || normalized.MaxTokens != 100 {
		t.Fatalf("request fields not preserved: %+v", normalized)
	}
	if normalized.Messages[0].ToolCalls[0].Name != "lookup" || normalized.Tools[0].Name != "lookup" {
		t.Fatalf("tool calls not mapped: %+v", normalized)
	}
}

func TestDecodeOpenAIRejectsMultimodalContent(t *testing.T) {
	_, err := DecodeOpenAIRequest([]byte(`{"model":"x","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`))
	if !errors.Is(err, ErrUnsupportedMultimodal) {
		t.Fatalf("expected unsupported multimodal error, got %v", err)
	}
}

func TestAnthropicToNormalizedWithSystemAndToolUse(t *testing.T) {
	req := AnthropicRequest{
		Model: "claude-sonnet",
		System: "be brief",
		Messages: []AnthropicMessage{{Role: "assistant", Content: []AnthropicContent{{Type: "text", Text: "checking"}, {Type: "tool_use", ID: "toolu_1", Name: "lookup", Input: []byte(`{"q":"x"}`)}}}},
		Tools: []AnthropicTool{{Name: "lookup", InputSchema: []byte(`{"type":"object"}`)}},
	}

	normalized, err := AnthropicToNormalized(req)
	if err != nil {
		t.Fatalf("AnthropicToNormalized returned error: %v", err)
	}
	if len(normalized.Messages) != 2 || normalized.Messages[0].Role != "system" || normalized.Messages[0].Content != "be brief" {
		t.Fatalf("system message not mapped: %+v", normalized.Messages)
	}
	if normalized.Messages[1].Content != "checking" || normalized.Messages[1].ToolCalls[0].ID != "toolu_1" {
		t.Fatalf("anthropic content not mapped: %+v", normalized.Messages[1])
	}
}

func TestAnthropicRejectsUnsupportedBlock(t *testing.T) {
	_, err := AnthropicToNormalized(AnthropicRequest{Messages: []AnthropicMessage{{Role: "user", Content: []AnthropicContent{{Type: "image"}}}}})
	if !errors.Is(err, ErrUnsupportedMultimodal) {
		t.Fatalf("expected unsupported block error, got %v", err)
	}
}

func TestNormalizedResponsesMapFormatsAndUsage(t *testing.T) {
	req := &models.NormalizedRequest{Model: "combo"}
	resp := &models.NormalizedResponse{Content: "done", FinishReason: models.FinishReasonToolCalls, Usage: models.NormalizedUsage{PromptTokens: 3, CompletionTokens: 5, TotalTokens: 8}, ToolCalls: []models.NormalizedToolCall{{ID: "call_1", Name: "lookup", Arguments: `{"q":"x"}`}}}

	openai := NormalizedToOpenAI(req, resp)
	if openai.Choices[0].FinishReason != "tool_calls" || openai.Usage.TotalTokens != 8 || openai.Choices[0].Message.ToolCalls[0].Function.Name != "lookup" {
		t.Fatalf("openai response not mapped: %+v", openai)
	}

	anthropic := NormalizedToAnthropic(req, resp)
	if anthropic.StopReason != "tool_use" || anthropic.Usage.InputTokens != 3 || anthropic.Content[1].Name != "lookup" {
		t.Fatalf("anthropic response not mapped: %+v", anthropic)
	}
}

func TestStreamingMappings(t *testing.T) {
	chunk := OpenAIResponse{Choices: []OpenAIChoice{{Delta: &OpenAIDelta{Content: "hel"}, FinishReason: "stop"}}}
	normalized := OpenAIStreamToNormalized(chunk)
	if normalized.Content != "hel" || normalized.FinishReason != models.FinishReasonStop {
		t.Fatalf("openai stream not normalized: %+v", normalized)
	}

	events := NormalizedStreamToAnthropic(normalized)
	if len(events) != 3 || events[0].Delta.Text != "hel" || events[1].Delta.StopReason != "end_turn" {
		t.Fatalf("normalized stream not mapped to anthropic: %+v", events)
	}

	anthropic := AnthropicStreamToNormalized(AnthropicStreamEvent{Type: "content_block_delta", Delta: &AnthropicDelta{Type: "text_delta", Text: "lo"}, Usage: &AnthropicUsage{InputTokens: 1, OutputTokens: 2}})
	if anthropic.Content != "lo" || anthropic.Usage.TotalTokens != 3 {
		t.Fatalf("anthropic stream not normalized: %+v", anthropic)
	}
}
