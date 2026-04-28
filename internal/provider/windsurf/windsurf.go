package windsurf

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
)

const (
	windsurfAPIEndpoint = "https://api.wavespeed.ai/v1/chat/completions"
	windsurfTimeout     = 60 * time.Second
)

type WindsurfProvider struct {
	client *http.Client
}

type windsurfTokenData struct {
	APIKey  string `json:"api_key"`
	KeyID   string `json:"key_id"`
	KeyName string `json:"key_name"`
}

func init() {
	provider.ProviderRegistry["windsurf"] = &WindsurfProvider{
		client: &http.Client{
			Timeout: windsurfTimeout,
		},
	}
}

func (p *WindsurfProvider) Name() string {
	return "Windsurf"
}

func (p *WindsurfProvider) Tier() string {
	return "windsurf"
}

func (p *WindsurfProvider) SupportsModel(modelID string) bool {
	info, ok := models.GetModelInfo(modelID)
	if !ok {
		return false
	}
	return info.Provider == models.ProviderWindsurf
}

func (p *WindsurfProvider) MapModel(modelID string) string {
	return modelID
}

func (p *WindsurfProvider) extractAPIKey(account *models.Account) (string, error) {
	if account.Token == "" {
		return "", fmt.Errorf("account token is empty")
	}

	var tokenData windsurfTokenData
	if err := json.Unmarshal([]byte(account.Token), &tokenData); err != nil {
		return "", fmt.Errorf("failed to parse token JSON: %w", err)
	}

	if tokenData.APIKey == "" {
		return "", fmt.Errorf("api_key not found in token data")
	}

	return tokenData.APIKey, nil
}

func (p *WindsurfProvider) SendChatCompletion(ctx context.Context, account *models.Account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	apiKey, err := p.extractAPIKey(account)
	if err != nil {
		return nil, fmt.Errorf("token extraction failed: %w", err)
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", windsurfAPIEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "enowXGateway/1.0.0")

	resp, err := p.client.Do(httpReq)
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

func (p *WindsurfProvider) SendChatCompletionStream(ctx context.Context, account *models.Account, req *provider.ChatRequest, writer http.ResponseWriter) error {
	apiKey, err := p.extractAPIKey(account)
	if err != nil {
		return fmt.Errorf("token extraction failed: %w", err)
	}

	req.Stream = true
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", windsurfAPIEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "enowXGateway/1.0.0")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
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

func (p *WindsurfProvider) ValidateAccount(ctx context.Context, account *models.Account) error {
	apiKey, err := p.extractAPIKey(account)
	if err != nil {
		return err
	}

	testReq := &provider.ChatRequest{
		Model: "windsurf-gpt-5",
		Messages: []provider.ChatMessage{
			{Role: "user", Content: "test"},
		},
		MaxTokens: 1,
	}

	reqBody, err := json.Marshal(testReq)
	if err != nil {
		return fmt.Errorf("failed to marshal test request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", windsurfAPIEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "enowXGateway/1.0.0")

	resp, err := p.client.Do(httpReq)
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

func (p *WindsurfProvider) GetCredits(ctx context.Context, account *models.Account) (used float64, total float64, err error) {
	return 0, 0, nil
}
