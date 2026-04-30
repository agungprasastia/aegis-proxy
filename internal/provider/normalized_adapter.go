package provider

import (
	"github.com/aegis-proxy/aegis/internal/anthropic"
	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/openai"
)

func OpenAIToNormalized(req *openai.ChatCompletionRequest) *models.NormalizedRequest {
	if req == nil {
		return nil
	}

	messages := make([]models.NormalizedMessage, len(req.Messages))
	for i, message := range req.Messages {
		messages[i] = models.NormalizedMessage{
			Role:      message.Role,
			Content:   message.Content,
			ToolCalls: openAIToolCallsToNormalized(message.ToolCalls),
		}
	}

	return &models.NormalizedRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Tools:       openAIToolCallsToNormalized(req.Tools),
		Stream:      req.Stream,
	}
}

func NormalizedToOpenAI(req *models.NormalizedRequest) *openai.ChatCompletionRequest {
	if req == nil {
		return nil
	}

	messages := make([]openai.ChatMessage, len(req.Messages))
	for i, message := range req.Messages {
		messages[i] = openai.ChatMessage{
			Role:      message.Role,
			Content:   message.Content,
			ToolCalls: normalizedToolCallsToOpenAI(message.ToolCalls),
		}
	}

	return &openai.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Tools:       normalizedToolCallsToOpenAI(req.Tools),
		Stream:      req.Stream,
	}
}

func AnthropicToNormalized(req *anthropic.MessageRequest) *models.NormalizedRequest {
	if req == nil {
		return nil
	}

	messages := make([]models.NormalizedMessage, len(req.Messages))
	for i, message := range req.Messages {
		messages[i] = models.NormalizedMessage{
			Role:      message.Role,
			Content:   message.Content,
			ToolCalls: anthropicToolUsesToNormalized(message.ToolCalls),
		}
	}

	return &models.NormalizedRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Tools:       anthropicToolUsesToNormalized(req.Tools),
		Stream:      req.Stream,
	}
}

func NormalizedToAnthropic(req *models.NormalizedRequest) *anthropic.MessageRequest {
	if req == nil {
		return nil
	}

	messages := make([]anthropic.Message, len(req.Messages))
	for i, message := range req.Messages {
		messages[i] = anthropic.Message{
			Role:      message.Role,
			Content:   message.Content,
			ToolCalls: normalizedToolCallsToAnthropic(message.ToolCalls),
		}
	}

	return &anthropic.MessageRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Tools:       normalizedToolCallsToAnthropic(req.Tools),
		Stream:      req.Stream,
	}
}

func openAIToolCallsToNormalized(toolCalls []openai.ToolCall) []models.NormalizedToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	normalized := make([]models.NormalizedToolCall, len(toolCalls))
	for i, toolCall := range toolCalls {
		normalized[i] = models.NormalizedToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Name,
			Arguments: toolCall.Arguments,
		}
	}
	return normalized
}

func normalizedToolCallsToOpenAI(toolCalls []models.NormalizedToolCall) []openai.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	openAIToolCalls := make([]openai.ToolCall, len(toolCalls))
	for i, toolCall := range toolCalls {
		openAIToolCalls[i] = openai.ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Name,
			Arguments: toolCall.Arguments,
		}
	}
	return openAIToolCalls
}

func anthropicToolUsesToNormalized(toolUses []anthropic.ToolUse) []models.NormalizedToolCall {
	if len(toolUses) == 0 {
		return nil
	}

	normalized := make([]models.NormalizedToolCall, len(toolUses))
	for i, toolUse := range toolUses {
		normalized[i] = models.NormalizedToolCall{
			ID:        toolUse.ID,
			Name:      toolUse.Name,
			Arguments: toolUse.Arguments,
		}
	}
	return normalized
}

func normalizedToolCallsToAnthropic(toolCalls []models.NormalizedToolCall) []anthropic.ToolUse {
	if len(toolCalls) == 0 {
		return nil
	}

	toolUses := make([]anthropic.ToolUse, len(toolCalls))
	for i, toolCall := range toolCalls {
		toolUses[i] = anthropic.ToolUse{
			ID:        toolCall.ID,
			Name:      toolCall.Name,
			Arguments: toolCall.Arguments,
		}
	}
	return toolUses
}
