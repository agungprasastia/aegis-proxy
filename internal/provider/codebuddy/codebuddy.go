package codebuddy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
	"github.com/aegis-proxy/aegis/internal/proxypool"
)

const (
	codeBuddyAPIEndpoint = "https://www.codebuddy.ai/v2/plugin/chat/completions"
	codeBuddyTimeout     = 60 * time.Second
)

type CodeBuddyProvider struct {
	client    *http.Client
	proxyPool *proxypool.ProxyPool
}

type codeBuddyTokenData struct {
	APIKey string `json:"api_key"`
	State  string `json:"state"`
}

func init() {
	provider.ProviderRegistry["codebuddy"] = &CodeBuddyProvider{
		client: &http.Client{
			Timeout: codeBuddyTimeout,
		},
	}
}

func (p *CodeBuddyProvider) Name() string {
	return "CodeBuddy"
}

func (p *CodeBuddyProvider) Tier() string {
	return "max"
}

func (p *CodeBuddyProvider) SupportsModel(modelID string) bool {
	info, ok := models.GetModelInfo(modelID)
	if !ok {
		return false
	}
	return info.Provider == models.ProviderCodeBuddy && info.Tier == "max"
}

func (p *CodeBuddyProvider) MapModel(modelID string) string {
	return modelID
}

func (p *CodeBuddyProvider) SetProxyPool(pool *proxypool.ProxyPool) {
	p.proxyPool = pool
}

func (p *CodeBuddyProvider) getClient() *http.Client {
	if p.proxyPool != nil {
		proxy := p.proxyPool.GetProxy("codebuddy")
		if proxy != nil && proxy.URL != "" {
			parsedURL, err := url.Parse(proxy.URL)
			if err == nil {
				return &http.Client{
					Timeout: codeBuddyTimeout,
					Transport: &http.Transport{
						Proxy: http.ProxyURL(parsedURL),
					},
				}
			}
		}
	}
	if p.client == nil {
		p.client = &http.Client{
			Timeout: codeBuddyTimeout,
		}
	}
	return p.client
}

func (p *CodeBuddyProvider) extractAPIKey(account *models.Account) (string, error) {
	if account.Token == "" {
		return "", fmt.Errorf("account token is empty")
	}

	var tokenData codeBuddyTokenData
	if err := json.Unmarshal([]byte(account.Token), &tokenData); err != nil {
		return "", fmt.Errorf("failed to parse token JSON: %w", err)
	}

	if tokenData.APIKey == "" {
		return "", fmt.Errorf("api_key not found in token data")
	}

	return tokenData.APIKey, nil
}

func (p *CodeBuddyProvider) SendChatCompletion(ctx context.Context, account *models.Account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	apiKey, err := p.extractAPIKey(account)
	if err != nil {
		return nil, fmt.Errorf("token extraction failed: %w", err)
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", codeBuddyAPIEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "enowXGateway/1.0.0")

	resp, err := p.getClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed: invalid or expired api_key")
	}
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limit exceeded")
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("server error: status %d", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var chatResp provider.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

func (p *CodeBuddyProvider) SendChatCompletionStream(ctx context.Context, account *models.Account, req *provider.ChatRequest, writer http.ResponseWriter) error {
	apiKey, err := p.extractAPIKey(account)
	if err != nil {
		return fmt.Errorf("token extraction failed: %w", err)
	}

	req.Stream = true
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", codeBuddyAPIEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "enowXGateway/1.0.0")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.getClient().Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("authentication failed: invalid or expired api_key")
	}
	if resp.StatusCode == 429 {
		return fmt.Errorf("rate limit exceeded")
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error: status %d", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")

	flusher, ok := writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			if data == "[DONE]" {
				fmt.Fprintf(writer, "data: [DONE]\n\n")
				flusher.Flush()
				break
			}

			fmt.Fprintf(writer, "data: %s\n\n", data)
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream reading error: %w", err)
	}

	return nil
}

func (p *CodeBuddyProvider) ValidateAccount(ctx context.Context, account *models.Account) error {
	apiKey, err := p.extractAPIKey(account)
	if err != nil {
		return err
	}

	testReq := &provider.ChatRequest{
		Model: "claude-opus-4.6",
		Messages: []provider.ChatMessage{
			{Role: "user", Content: "test"},
		},
		MaxTokens: 1,
	}

	reqBody, err := json.Marshal(testReq)
	if err != nil {
		return fmt.Errorf("failed to marshal test request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", codeBuddyAPIEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "enowXGateway/1.0.0")

	resp, err := p.getClient().Do(httpReq)
	if err != nil {
		return fmt.Errorf("validation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("invalid or expired api_key")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("validation failed: status %d", resp.StatusCode)
	}

	return nil
}

func (p *CodeBuddyProvider) GetCredits(ctx context.Context, account *models.Account) (used float64, total float64, err error) {
	// CodeBuddy doesn't expose a credits endpoint, return 0
	return 0, 0, nil
}
