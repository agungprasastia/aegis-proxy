package translate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aegis-proxy/aegis/internal/models"
)

type GeminiRequest struct {
	Contents         []GeminiContent          `json:"contents"`
	SystemInstruction *GeminiContent         `json:"system_instruction,omitempty"`
	Tools            []GeminiTool             `json:"tools,omitempty"`
	GenerationConfig *GeminiGenerationConfig  `json:"generationConfig,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text         string          `json:"text,omitempty"`
	FunctionCall *GeminiFuncCall `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFuncResponse `json:"functionResponse,omitempty"`
}

type GeminiFuncCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type GeminiFuncResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response,omitempty"`
}

type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type GeminiFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type GeminiGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

type GeminiResponse struct {
	Candidates     []GeminiCandidate `json:"candidates"`
	UsageMetadata  GeminiUsage        `json:"usageMetadata"`
}

type GeminiCandidate struct {
	Content      GeminiContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
}

type GeminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

func NormalizedToGemini(req *models.NormalizedRequest) (GeminiRequest, error) {
	if req == nil {
		return GeminiRequest{}, nil
	}
	out := GeminiRequest{GenerationConfig: &GeminiGenerationConfig{MaxOutputTokens: req.MaxTokens, Temperature: req.Temperature}}
	for _, message := range req.Messages {
		if strings.TrimSpace(message.Content) != "" && (strings.HasPrefix(strings.TrimSpace(message.Content), "[") || strings.Contains(message.Content, "image_url") || strings.Contains(message.Content, "input_audio")) {
			return GeminiRequest{}, ErrUnsupportedMultimodal
		}
		content := GeminiContent{Role: geminiRole(message.Role)}
		if message.Role == "system" {
			out.SystemInstruction = &GeminiContent{Parts: []GeminiPart{{Text: message.Content}}}
			continue
		}
		if message.Content != "" {
			content.Parts = append(content.Parts, GeminiPart{Text: message.Content})
		}
		for _, call := range message.ToolCalls {
			content.Parts = append(content.Parts, GeminiPart{FunctionCall: &GeminiFuncCall{Name: call.Name, Args: json.RawMessage(call.Arguments)}})
		}
		if len(content.Parts) > 0 {
			out.Contents = append(out.Contents, content)
		}
	}
	if len(req.Tools) > 0 {
		decls := make([]GeminiFunctionDeclaration, 0, len(req.Tools))
		for _, tool := range req.Tools {
			decls = append(decls, GeminiFunctionDeclaration{Name: tool.Name, Parameters: json.RawMessage(tool.Arguments)})
		}
		out.Tools = []GeminiTool{{FunctionDeclarations: decls}}
	}
	return out, nil
}

func GeminiToNormalized(resp GeminiResponse) (*models.NormalizedResponse, error) {
	if len(resp.Candidates) == 0 {
		return &models.NormalizedResponse{Usage: geminiUsageToNormalized(resp.UsageMetadata)}, nil
	}
	candidate := resp.Candidates[0]
	content := ""
	calls := []models.NormalizedToolCall{}
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			content += part.Text
		}
		if part.FunctionCall != nil {
			calls = append(calls, models.NormalizedToolCall{Name: part.FunctionCall.Name, Arguments: string(part.FunctionCall.Args)})
		}
		if part.FunctionResponse != nil {
			return nil, fmt.Errorf("%w: gemini function response in model output", ErrUnsupportedMultimodal)
		}
	}
	if len(calls) == 0 {
		calls = nil
	}
	return &models.NormalizedResponse{Content: content, ToolCalls: calls, FinishReason: geminiFinishToNormalized(candidate.FinishReason), Usage: geminiUsageToNormalized(resp.UsageMetadata)}, nil
}

func geminiRole(role string) string {
	switch role {
	case "assistant":
		return "model"
	case "tool":
		return "function"
	default:
		return "user"
	}
}

func geminiFinishToNormalized(reason string) models.FinishReason {
	switch reason {
	case "STOP":
		return models.FinishReasonStop
	case "MAX_TOKENS":
		return models.FinishReasonLength
	case "MALFORMED_FUNCTION_CALL":
		return models.FinishReasonError
	default:
		if strings.Contains(reason, "SAFETY") || strings.Contains(reason, "RECITATION") {
			return models.FinishReasonError
		}
		return ""
	}
}

func geminiUsageToNormalized(usage GeminiUsage) models.NormalizedUsage {
	total := usage.TotalTokenCount
	if total == 0 {
		total = usage.PromptTokenCount + usage.CandidatesTokenCount
	}
	return models.NormalizedUsage{PromptTokens: usage.PromptTokenCount, CompletionTokens: usage.CandidatesTokenCount, TotalTokens: total}
}
