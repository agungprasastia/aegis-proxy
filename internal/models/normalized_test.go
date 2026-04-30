package models_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aegis-proxy/aegis/internal/anthropic"
	"github.com/aegis-proxy/aegis/internal/openai"
	"github.com/aegis-proxy/aegis/internal/provider"
)

func TestOpenAIRoundTripNormalized(t *testing.T) {
	req := &openai.ChatCompletionRequest{
		Model:       "claude-sonnet-4.5",
		MaxTokens:   1024,
		Temperature: 0.7,
		Stream:      true,
		Tools: []openai.ToolCall{
			{ID: "tool-1", Name: "search", Arguments: `{"query":"aegis"}`},
		},
		Messages: []openai.ChatMessage{
			{Role: "system", Content: "Be concise."},
			{Role: "user", Content: "Find docs."},
			{
				Role:    "assistant",
				Content: "Calling tool.",
				ToolCalls: []openai.ToolCall{
					{ID: "call-1", Name: "search", Arguments: `{"query":"docs"}`},
				},
			},
		},
	}

	got := provider.NormalizedToOpenAI(provider.OpenAIToNormalized(req))
	assertJSONEqual(t, req, got)
}

func TestAnthropicRoundTripNormalized(t *testing.T) {
	req := &anthropic.MessageRequest{
		Model:       "claude-sonnet-4",
		MaxTokens:   2048,
		Temperature: 0.2,
		Stream:      true,
		Tools: []anthropic.ToolUse{
			{ID: "tool-1", Name: "lookup", Arguments: `{"id":"123"}`},
		},
		Messages: []anthropic.Message{
			{Role: "user", Content: "Lookup item."},
			{
				Role:    "assistant",
				Content: "Using lookup.",
				ToolCalls: []anthropic.ToolUse{
					{ID: "call-1", Name: "lookup", Arguments: `{"id":"456"}`},
				},
			},
		},
	}

	got := provider.NormalizedToAnthropic(provider.AnthropicToNormalized(req))
	assertJSONEqual(t, req, got)
}

func TestUnsupportedMultimodalFieldsRejected(t *testing.T) {
	data := []byte(`{
		"model":"claude-sonnet-4",
		"messages":[{
			"role":"user",
			"content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,abc"}}]
		}]
	}`)

	var req openai.ChatCompletionRequest
	err := json.Unmarshal(data, &req)
	if err == nil {
		t.Fatal("expected multimodal content to be rejected")
	}
	if !strings.Contains(err.Error(), "cannot unmarshal array") || !strings.Contains(err.Error(), "string") {
		t.Fatalf("expected clear string content error, got %q", err.Error())
	}
}

func assertJSONEqual(t *testing.T, want, got any) {
	t.Helper()

	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	if string(wantJSON) != string(gotJSON) {
		t.Fatalf("round trip mismatch\nwant: %s\n got: %s", wantJSON, gotJSON)
	}
}
