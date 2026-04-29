package kiro

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
	"github.com/aegis-proxy/aegis/internal/proxypool"
)

const (
	kiroAPIEndpoint   = "https://q.us-east-1.amazonaws.com/"
	kiroUsageEndpoint = "https://q.us-east-1.amazonaws.com/getUsageLimits"
	kiroTimeout       = 120 * time.Second
)

type KiroProvider struct {
	client    *http.Client
	proxyPool *proxypool.ProxyPool
}

type kiroTokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ProfileARN   string `json:"profile_arn"`
	ExpiresAt    string `json:"expires_at"`
	ExpiresIn    string `json:"expires_in"`
}

// Amazon Q request format (matches official AWS SDK structure)
type kiroRequest struct {
	ConversationState kiroConversationState `json:"conversationState"`
	ProfileARN        string                `json:"profileArn,omitempty"`
}

type kiroConversationState struct {
	ConversationID  string             `json:"conversationId,omitempty"`
	CurrentMessage  kiroCurrentMessage `json:"currentMessage"`
	ChatTriggerType string             `json:"chatTriggerType"`
	History         []kiroChatMessage  `json:"history,omitempty"`
}

type kiroCurrentMessage struct {
	UserInputMessage kiroUserInputMessage `json:"userInputMessage"`
}

type kiroUserInputMessage struct {
	Content                 string                  `json:"content"`
	UserInputMessageContext *kiroMessageContext      `json:"userInputMessageContext,omitempty"`
	Origin                  string                  `json:"origin,omitempty"`
	ModelID                 string                  `json:"modelId,omitempty"`
}

type kiroMessageContext struct {
	EnvState *kiroEnvState `json:"envState,omitempty"`
}

type kiroEnvState struct {
	OperatingSystem        string `json:"operatingSystem,omitempty"`
	CurrentWorkingDirectory string `json:"currentWorkingDirectory,omitempty"`
}

type kiroChatMessage struct {
	UserInputMessage         *kiroUserInputMessage         `json:"userInputMessage,omitempty"`
	AssistantResponseMessage *kiroAssistantResponseMessage  `json:"assistantResponseMessage,omitempty"`
}

type kiroAssistantResponseMessage struct {
	MessageID string `json:"messageId"`
	Content   string `json:"content"`
}

type kiroUsageResponse struct {
	UsageBreakdownList []struct {
		UsageLimit   float64 `json:"usageLimit"`
		CurrentUsage float64 `json:"currentUsage"`
		FreeTrialInfo struct {
			FreeTrialStatus string  `json:"freeTrialStatus"`
			UsageLimit      float64 `json:"usageLimit"`
			CurrentUsage    float64 `json:"currentUsage"`
		} `json:"freeTrialInfo"`
		Bonuses []struct {
			UsageLimit   float64 `json:"usageLimit"`
			CurrentUsage float64 `json:"currentUsage"`
		} `json:"bonuses"`
	} `json:"usageBreakdownList"`
}

func init() {
	provider.ProviderRegistry["kiro"] = &KiroProvider{
		client: &http.Client{
			Timeout: kiroTimeout,
		},
	}
}

func (p *KiroProvider) Name() string {
	return "Kiro"
}

func (p *KiroProvider) Tier() string {
	return "standard"
}

func (p *KiroProvider) SupportsModel(modelID string) bool {
	info, ok := models.GetModelInfo(modelID)
	if !ok {
		return false
	}
	return info.Provider == models.ProviderKiro && info.Tier == "standard"
}

func (p *KiroProvider) MapModel(modelID string) string {
	return modelID
}

func (p *KiroProvider) SetProxyPool(pool *proxypool.ProxyPool) {
	p.proxyPool = pool
}

func (p *KiroProvider) getClient() *http.Client {
	if p.proxyPool != nil {
		proxy := p.proxyPool.GetProxy("kiro")
		if proxy != nil && proxy.URL != "" {
			parsedURL, err := url.Parse(proxy.URL)
			if err == nil {
				return &http.Client{
					Timeout: kiroTimeout,
					Transport: &http.Transport{
						Proxy: http.ProxyURL(parsedURL),
					},
				}
			}
		}
	}
	if p.client == nil {
		p.client = &http.Client{
			Timeout: kiroTimeout,
		}
	}
	return p.client
}

func (p *KiroProvider) extractTokenData(account *models.Account) (*kiroTokenData, error) {
	if account.Token == "" {
		return nil, fmt.Errorf("account token is empty")
	}

	var tokenData kiroTokenData
	if err := json.Unmarshal([]byte(account.Token), &tokenData); err != nil {
		return nil, fmt.Errorf("failed to parse token JSON: %w", err)
	}

	if tokenData.AccessToken == "" {
		return nil, fmt.Errorf("access_token not found in token data")
	}

	return &tokenData, nil
}

func (p *KiroProvider) extractAccessToken(account *models.Account) (string, error) {
	td, err := p.extractTokenData(account)
	if err != nil {
		return "", err
	}
	return td.AccessToken, nil
}

// generateUUID generates a random UUID v4 string
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// mapModelToKiro maps our model IDs to Amazon Q model IDs
func mapModelToKiro(modelID string) string {
	modelMap := map[string]string{
		"auto":              "",
		"claude-sonnet-4.5": "claude-sonnet-4-5-v2",
		"claude-sonnet-4":   "claude-sonnet-4",
		"claude-haiku-4.5":  "claude-haiku-4-5-v1",
		"deepseek-3.2":      "deepseek-r1",
		"minimax-m2.5":      "minimax-m1",
		"glm-5":             "glm-4-plus",
		"qwen3-coder-next":  "qwen2-5-max",
	}
	if mapped, ok := modelMap[modelID]; ok {
		return mapped
	}
	return modelID
}

// buildKiroPayload converts OpenAI-style ChatRequest to Amazon Q format
func buildKiroPayload(req *provider.ChatRequest, profileARN string, modelID string) (*kiroRequest, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	// Extract the last user message as currentMessage
	var userMessage string
	var history []kiroChatMessage

	// Collect system prompt
	var systemPrompt string

	// Build history from all messages except the last user message
	for i, msg := range req.Messages {
		if i == len(req.Messages)-1 && msg.Role == "user" {
			userMessage = msg.Content
			continue
		}

		switch msg.Role {
		case "user":
			history = append(history, kiroChatMessage{
				UserInputMessage: &kiroUserInputMessage{
					Content: msg.Content,
					UserInputMessageContext: &kiroMessageContext{
						EnvState: &kiroEnvState{
							OperatingSystem:         "windows",
							CurrentWorkingDirectory: "/",
						},
					},
					Origin: "CLI",
				},
			})
		case "assistant":
			history = append(history, kiroChatMessage{
				AssistantResponseMessage: &kiroAssistantResponseMessage{
					MessageID: generateUUID(),
					Content:   msg.Content,
				},
			})
		case "system":
			systemPrompt = msg.Content
		}
	}

	// If no explicit user message found at end, use last message
	if userMessage == "" {
		lastMsg := req.Messages[len(req.Messages)-1]
		userMessage = lastMsg.Content
		// Remove from history if it was added
		if len(history) > 0 {
			history = history[:len(history)-1]
		}
	}

	// Prepend system prompt to user message if present
	if systemPrompt != "" {
		userMessage = "--- SYSTEM PROMPT BEGIN ---\n" + systemPrompt + "\n--- SYSTEM PROMPT END ---\n\n" + userMessage
	}

	payload := &kiroRequest{
		ConversationState: kiroConversationState{
			ConversationID: generateUUID(),
			CurrentMessage: kiroCurrentMessage{
				UserInputMessage: kiroUserInputMessage{
					Content: userMessage,
					UserInputMessageContext: &kiroMessageContext{
						EnvState: &kiroEnvState{
							OperatingSystem:         "windows",
							CurrentWorkingDirectory: "/",
						},
					},
					Origin:  "CLI",
					ModelID: modelID,
				},
			},
			ChatTriggerType: "MANUAL",
			History:         history,
		},
		ProfileARN: profileARN,
	}

	return payload, nil
}

func (p *KiroProvider) SendChatCompletion(ctx context.Context, account *models.Account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	tokenData, err := p.extractTokenData(account)
	if err != nil {
		return nil, fmt.Errorf("token extraction failed: %w", err)
	}

	// Map model ID for Amazon Q
	modelID := mapModelToKiro(req.Model)

	payload, err := buildKiroPayload(req, tokenData.ProfileARN, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to build Kiro payload: %w", err)
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send plain JSON request (response comes back as Event Stream binary)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", kiroAPIEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
	httpReq.Header.Set("Content-Type", "application/x-amz-json-1.0")
	httpReq.Header.Set("X-Amz-Target", "AmazonCodeWhispererStreamingService.GenerateAssistantResponse")
	httpReq.Header.Set("Accept", "*/*")
	httpReq.Header.Set("User-Agent", "aws-sdk-rust/1.3.9 ua/2.1 api/codewhispererstreaming/0.1.11582 os/windows lang/go app/AmazonQ-For-CLI")
	httpReq.Header.Set("X-Amzn-Codewhisperer-Optout", "true")
	httpReq.Header.Set("Amz-Sdk-Request", "attempt=1; max=3")
	if tokenData.ProfileARN != "" {
		httpReq.Header.Set("x-amzn-codewhisperer-profilearn", tokenData.ProfileARN)
	}

	resp, err := p.getClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("authentication failed: invalid or expired token (status %d)", resp.StatusCode)
	}
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limit exceeded")
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("server error: status %d", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body[:min(len(body), 500)]))
	}

	// Decode AWS Event Stream binary response
	messages, err := DecodeEventStream(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode event stream: %w", err)
	}

	content, inputTokens, outputTokens, _ := ParseKiroEvents(messages)
	if content == "" {
		return nil, fmt.Errorf("empty response from Kiro API")
	}

	// Build OpenAI-compatible response
	chatResp := &provider.ChatResponse{
		ID:      fmt.Sprintf("chatcmpl-kiro-%d", time.Now().UnixMilli()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []provider.Choice{
			{
				Index: 0,
				Message: provider.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
	}

	chatResp.Usage = provider.Usage{
		PromptTokens:     inputTokens,
		CompletionTokens: outputTokens,
		TotalTokens:      inputTokens + outputTokens,
	}

	return chatResp, nil
}

func (p *KiroProvider) SendChatCompletionStream(ctx context.Context, account *models.Account, req *provider.ChatRequest, writer http.ResponseWriter) error {
	tokenData, err := p.extractTokenData(account)
	if err != nil {
		return fmt.Errorf("token extraction failed: %w", err)
	}

	// Map model ID for Amazon Q
	modelID := mapModelToKiro(req.Model)

	payload, err := buildKiroPayload(req, tokenData.ProfileARN, modelID)
	if err != nil {
		return fmt.Errorf("failed to build Kiro payload: %w", err)
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send plain JSON request (response comes back as Event Stream binary)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", kiroAPIEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
	httpReq.Header.Set("Content-Type", "application/x-amz-json-1.0")
	httpReq.Header.Set("X-Amz-Target", "AmazonCodeWhispererStreamingService.GenerateAssistantResponse")
	httpReq.Header.Set("Accept", "*/*")
	httpReq.Header.Set("User-Agent", "aws-sdk-rust/1.3.9 ua/2.1 api/codewhispererstreaming/0.1.11582 os/windows lang/go app/AmazonQ-For-CLI")
	httpReq.Header.Set("X-Amzn-Codewhisperer-Optout", "true")
	httpReq.Header.Set("Amz-Sdk-Request", "attempt=1; max=3")
	if tokenData.ProfileARN != "" {
		httpReq.Header.Set("x-amzn-codewhisperer-profilearn", tokenData.ProfileARN)
	}

	resp, err := p.getClient().Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed: invalid or expired token")
	}
	if resp.StatusCode == 429 {
		return fmt.Errorf("rate limit exceeded")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body[:min(len(body), 500)]))
	}

	// Stream OpenAI-compatible SSE events from AWS Event Stream binary response
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")

	flusher, ok := writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	streamID := fmt.Sprintf("chatcmpl-kiro-%d", time.Now().UnixMilli())

	// Decode AWS Event Stream messages and convert to OpenAI SSE
	messages, err := DecodeEventStream(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to decode event stream: %w", err)
	}

	for _, msg := range messages {
		eventType := msg.Headers[":event-type"]

		if eventType == "assistantResponseEvent" && len(msg.Payload) > 0 {
			var event struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(msg.Payload, &event); err == nil && event.Content != "" {
				chunk := map[string]interface{}{
					"id":      streamID,
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   req.Model,
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"delta": map[string]string{
								"content": event.Content,
							},
						},
					},
				}
				chunkJSON, _ := json.Marshal(chunk)
				fmt.Fprintf(writer, "data: %s\n\n", chunkJSON)
				flusher.Flush()
			}
		}
	}

	// Send final [DONE] event
	fmt.Fprintf(writer, "data: [DONE]\n\n")
	flusher.Flush()

	return nil
}

func (p *KiroProvider) ValidateAccount(ctx context.Context, account *models.Account) error {
	accessToken, err := p.extractAccessToken(account)
	if err != nil {
		return err
	}

	tokenData, _ := p.extractTokenData(account)

	usageURL := kiroUsageEndpoint
	if tokenData != nil && tokenData.ProfileARN != "" {
		params := url.Values{}
		params.Add("origin", "AI_EDITOR")
		params.Add("resourceType", "AGENTIC_REQUEST")
		params.Add("profileArn", tokenData.ProfileARN)
		usageURL = kiroUsageEndpoint + "?" + params.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", usageURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "enowXGateway/1.0.0")

	resp, err := p.getClient().Do(httpReq)
	if err != nil {
		return fmt.Errorf("validation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("invalid or expired token")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("validation failed: status %d", resp.StatusCode)
	}

	return nil
}

func (p *KiroProvider) GetCredits(ctx context.Context, account *models.Account) (used float64, total float64, err error) {
	accessToken, err := p.extractAccessToken(account)
	if err != nil {
		return 0, 0, err
	}

	tokenData, _ := p.extractTokenData(account)

	usageURL := kiroUsageEndpoint
	if tokenData != nil && tokenData.ProfileARN != "" {
		params := url.Values{}
		params.Add("origin", "AI_EDITOR")
		params.Add("resourceType", "AGENTIC_REQUEST")
		params.Add("profileArn", tokenData.ProfileARN)
		usageURL = kiroUsageEndpoint + "?" + params.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", usageURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "enowXGateway/1.0.0")

	resp, err := p.getClient().Do(httpReq)
	if err != nil {
		return 0, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, 0, fmt.Errorf("failed to fetch credits: status %d", resp.StatusCode)
	}

	var usageResp kiroUsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&usageResp); err != nil {
		return 0, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(usageResp.UsageBreakdownList) == 0 {
		return 0, 0, nil
	}

	usage := usageResp.UsageBreakdownList[0]
	totalCredits := usage.UsageLimit
	usedCredits := usage.CurrentUsage

	if usage.FreeTrialInfo.FreeTrialStatus == "ACTIVE" {
		totalCredits += usage.FreeTrialInfo.UsageLimit
		usedCredits += usage.FreeTrialInfo.CurrentUsage
	}

	for _, bonus := range usage.Bonuses {
		totalCredits += bonus.UsageLimit
		usedCredits += bonus.CurrentUsage
	}

	return usedCredits, totalCredits, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
