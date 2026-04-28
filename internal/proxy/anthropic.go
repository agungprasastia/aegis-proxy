package proxy

type AnthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []OpenAIMessage    `json:"messages"`
	MaxTokens int                `json:"max_tokens,omitempty"`
	System    string             `json:"system,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

type AnthropicResponse struct {
	ID      string             `json:"id"`
	Type    string             `json:"type"`
	Role    string             `json:"role"`
	Content []AnthropicContent `json:"content"`
	Model   string             `json:"model"`
	Usage   AnthropicUsage     `json:"usage"`
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

type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func convertAnthropicToInternal(req *AnthropicRequest) *OpenAIChatRequest {
	openaiReq := &OpenAIChatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Stream:      req.Stream,
		MaxTokens:   req.MaxTokens,
		Temperature: 1.0,
	}
	
	if req.System != "" {
		systemMsg := OpenAIMessage{
			Role:    "system",
			Content: req.System,
		}
		openaiReq.Messages = append([]OpenAIMessage{systemMsg}, openaiReq.Messages...)
	}
	
	return openaiReq
}

func convertInternalToAnthropic(resp *OpenAIChatResponse) *AnthropicResponse {
	var content []AnthropicContent
	
	if len(resp.Choices) > 0 {
		content = []AnthropicContent{
			{
				Type: "text",
				Text: resp.Choices[0].Message.Content,
			},
		}
	}
	
	return &AnthropicResponse{
		ID:      resp.ID,
		Type:    "message",
		Role:    "assistant",
		Content: content,
		Model:   resp.Model,
		Usage: AnthropicUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
}

func convertOpenAIStreamToAnthropic(chunk *OpenAIStreamChunk) []AnthropicStreamEvent {
	var events []AnthropicStreamEvent
	
	if len(chunk.Choices) > 0 {
		choice := chunk.Choices[0]
		
		if choice.Delta != nil && choice.Delta.Content != "" {
			events = append(events, AnthropicStreamEvent{
				Type:  "content_block_delta",
				Index: 0,
				Delta: &AnthropicDelta{
					Type: "text_delta",
					Text: choice.Delta.Content,
				},
			})
		}
		
		if choice.FinishReason != "" {
			events = append(events, AnthropicStreamEvent{
				Type: "message_delta",
				Delta: &AnthropicDelta{
					StopReason: choice.FinishReason,
				},
			})
			
			events = append(events, AnthropicStreamEvent{
				Type: "message_stop",
			})
		}
	}
	
	return events
}
