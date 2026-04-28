package proxy

import "encoding/json"

type OpenAIChatRequest struct {
	Model           string          `json:"model"`
	Messages        []OpenAIMessage `json:"messages"`
	Stream          bool            `json:"stream,omitempty"`
	MaxTokens       int             `json:"max_tokens,omitempty"`
	Temperature     float64         `json:"temperature,omitempty"`
	Tools           []interface{}   `json:"tools,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
}

type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

type OpenAIStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message,omitempty"`
	Delta        *OpenAIDelta  `json:"delta,omitempty"`
	FinishReason string        `json:"finish_reason,omitempty"`
}

type OpenAIDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIModelList struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

type OpenAIModel struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	OwnedBy  string `json:"owned_by"`
	Permission []interface{} `json:"permission,omitempty"`
}

func convertToOpenAIRequest(data []byte) (*OpenAIChatRequest, error) {
	var req OpenAIChatRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func convertFromOpenAIResponse(resp *OpenAIChatResponse) ([]byte, error) {
	return json.Marshal(resp)
}
