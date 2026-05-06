package translate

import (
	"errors"
	"testing"

	"github.com/aegis-proxy/aegis/internal/models"
)

func TestNormalizedToGeminiTextAndTools(t *testing.T) {
	req := &models.NormalizedRequest{
		Model: "gemini-2.5-pro",
		Messages: []models.NormalizedMessage{{Role: "system", Content: "brief"}, {Role: "user", Content: "hello"}, {Role: "assistant", ToolCalls: []models.NormalizedToolCall{{Name: "lookup", Arguments: `{"q":"x"}`}}}},
		Tools: []models.NormalizedToolCall{{Name: "lookup", Arguments: `{"type":"object"}`}},
		MaxTokens: 64,
		Temperature: 0.2,
	}

	gemini, err := NormalizedToGemini(req)
	if err != nil {
		t.Fatalf("NormalizedToGemini error: %v", err)
	}
	if gemini.SystemInstruction == nil || gemini.SystemInstruction.Parts[0].Text != "brief" {
		t.Fatalf("system instruction not mapped: %+v", gemini.SystemInstruction)
	}
	if len(gemini.Contents) != 2 || gemini.Contents[1].Role != "model" || gemini.Contents[1].Parts[0].FunctionCall.Name != "lookup" {
		t.Fatalf("contents not mapped: %+v", gemini.Contents)
	}
	if len(gemini.Tools) != 1 || gemini.Tools[0].FunctionDeclarations[0].Name != "lookup" {
		t.Fatalf("tools not mapped: %+v", gemini.Tools)
	}
}

func TestNormalizedToGeminiRejectsUnsupportedPayload(t *testing.T) {
	_, err := NormalizedToGemini(&models.NormalizedRequest{Messages: []models.NormalizedMessage{{Role: "user", Content: `[{"type":"image_url"}]`}}})
	if !errors.Is(err, ErrUnsupportedMultimodal) {
		t.Fatalf("expected unsupported multimodal error, got %v", err)
	}
}

func TestGeminiToNormalizedTextToolUsage(t *testing.T) {
	resp, err := GeminiToNormalized(GeminiResponse{
		Candidates: []GeminiCandidate{{FinishReason: "MAX_TOKENS", Content: GeminiContent{Parts: []GeminiPart{{Text: "answer"}, {FunctionCall: &GeminiFuncCall{Name: "lookup", Args: []byte(`{"q":"x"}`)}}}}}},
		UsageMetadata: GeminiUsage{PromptTokenCount: 2, CandidatesTokenCount: 4, TotalTokenCount: 6},
	})
	if err != nil {
		t.Fatalf("GeminiToNormalized error: %v", err)
	}
	if resp.Content != "answer" || resp.FinishReason != models.FinishReasonLength || resp.Usage.TotalTokens != 6 {
		t.Fatalf("gemini response not mapped: %+v", resp)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "lookup" {
		t.Fatalf("tool call not mapped: %+v", resp.ToolCalls)
	}
}

func TestGeminiToNormalizedRejectsFunctionResponseOutput(t *testing.T) {
	_, err := GeminiToNormalized(GeminiResponse{Candidates: []GeminiCandidate{{Content: GeminiContent{Parts: []GeminiPart{{FunctionResponse: &GeminiFuncResponse{Name: "lookup"}}}}}}})
	if !errors.Is(err, ErrUnsupportedMultimodal) {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
