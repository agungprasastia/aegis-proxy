package canva

import (
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
	"github.com/google/uuid"
)

const (
	canvaAPIEndpoint   = "https://www.canva.com/_ajax/assistant/threads"
	canvaQuotaEndpoint = "https://www.canva.com/_ajax/quota/quota/get"
	canvaTimeout       = 90 * time.Second
	pollInterval       = 2 * time.Second
	maxPollAttempts    = 30
)

type CanvaProvider struct {
	client *http.Client
}

type canvaTokenData struct {
	CAZ        string `json:"caz"`
	CB         string `json:"cb"`
	CAU        string `json:"cau"`
	UserID     string `json:"user_id"`
	AllCookies string `json:"all_cookies"`
}

type canvaCreateThreadRequest struct {
	A  string                   `json:"A"`
	B  []map[string]interface{} `json:"B"`
	C  string                   `json:"C"`
	D  map[string]interface{}   `json:"D"`
	AQ string                   `json:"A?"`
}

type canvaThreadResponse struct {
	A  string                   `json:"A"`
	E  map[string]interface{}   `json:"e"`
	F  []map[string]interface{} `json:"f"`
	AQ string                   `json:"A?"`
}

type canvaQuotaRequest struct {
	A string `json:"A"`
	B string `json:"B"`
	C string `json:"C"`
}

type canvaQuotaResponse struct {
	A map[string]interface{} `json:"A"`
}

func init() {
	provider.ProviderRegistry["canva"] = &CanvaProvider{
		client: &http.Client{
			Timeout: canvaTimeout,
		},
	}
}

func (p *CanvaProvider) Name() string {
	return "Canva"
}

func (p *CanvaProvider) Tier() string {
	return "canva"
}

func (p *CanvaProvider) SupportsModel(modelID string) bool {
	info, ok := models.GetModelInfo(modelID)
	if !ok {
		return false
	}
	return info.Provider == models.ProviderCanva
}

func (p *CanvaProvider) MapModel(modelID string) string {
	return modelID
}

func (p *CanvaProvider) extractTokenData(account *models.Account) (*canvaTokenData, error) {
	if account.Token == "" {
		return nil, fmt.Errorf("account token is empty")
	}

	var tokenData canvaTokenData
	if err := json.Unmarshal([]byte(account.Token), &tokenData); err != nil {
		return nil, fmt.Errorf("failed to parse token JSON: %w", err)
	}

	if tokenData.CAZ == "" {
		return nil, fmt.Errorf("caz not found in token data")
	}

	return &tokenData, nil
}

func (p *CanvaProvider) buildCookieHeader(tokenData *canvaTokenData) (string, error) {
	if tokenData.AllCookies == "" {
		return fmt.Sprintf("CAZ=%s; CB=%s; CAU=%s", tokenData.CAZ, tokenData.CB, tokenData.CAU), nil
	}

	var allCookies map[string]string
	if err := json.Unmarshal([]byte(tokenData.AllCookies), &allCookies); err != nil {
		return "", fmt.Errorf("failed to parse all_cookies: %w", err)
	}

	var cookieParts []string
	for name, value := range allCookies {
		cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", name, value))
	}

	return strings.Join(cookieParts, "; "), nil
}

func (p *CanvaProvider) SendChatCompletion(ctx context.Context, account *models.Account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	tokenData, err := p.extractTokenData(account)
	if err != nil {
		return nil, fmt.Errorf("token extraction failed: %w", err)
	}

	cookieHeader, err := p.buildCookieHeader(tokenData)
	if err != nil {
		return nil, fmt.Errorf("failed to build cookie header: %w", err)
	}

	// Extract prompt from messages
	prompt := ""
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			prompt = msg.Content
			break
		}
	}
	if prompt == "" {
		return nil, fmt.Errorf("no user message found in request")
	}

	// Enhance prompt for image generation
	if !strings.Contains(strings.ToLower(prompt), "generate") &&
		!strings.Contains(strings.ToLower(prompt), "create") &&
		!strings.Contains(strings.ToLower(prompt), "draw") {
		prompt = "Generate an image of " + prompt
	}

	// Create thread
	threadID, err := p.createThread(ctx, tokenData, cookieHeader, prompt)
	if err != nil {
		return nil, err
	}

	// Poll for results
	images, err := p.pollThreadResults(ctx, tokenData, cookieHeader, threadID)
	if err != nil {
		return nil, err
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images generated")
	}

	// Build OpenAI-compatible response
	content := fmt.Sprintf("Generated %d image(s):\n", len(images))
	for i, url := range images {
		content += fmt.Sprintf("%d. %s\n", i+1, url)
	}

	chatResp := &provider.ChatResponse{
		ID:      fmt.Sprintf("canva-%s", threadID),
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

	return chatResp, nil
}

func (p *CanvaProvider) createThread(ctx context.Context, tokenData *canvaTokenData, cookieHeader, prompt string) (string, error) {
	threadReq := canvaCreateThreadRequest{
		A: strings.ToUpper(uuid.New().String()[:26]),
		B: []map[string]interface{}{
			{
				"A?": "A",
				"A":  prompt,
				"L":  prompt[:min(50, len(prompt))],
			},
		},
		C: uuid.New().String(),
		D: map[string]interface{}{
			"D": "D",
			"G": map[string]string{"A?": "E"},
			"H": "C",
			"J": "UTC",
		},
		AQ: "G",
	}

	reqBody, err := json.Marshal(threadReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal thread request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", canvaAPIEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Cookie", cookieHeader)
	httpReq.Header.Set("Content-Type", "application/json;charset=UTF-8")
	httpReq.Header.Set("Origin", "https://www.canva.com")
	httpReq.Header.Set("Referer", "https://www.canva.com/ai")
	httpReq.Header.Set("x-canva-authz", tokenData.CAZ)
	httpReq.Header.Set("x-canva-brand", tokenData.CB)
	httpReq.Header.Set("x-canva-user", tokenData.UserID)
	httpReq.Header.Set("x-canva-active-user", tokenData.CAU)
	httpReq.Header.Set("x-canva-accept-prefix", "no-prefix")
	httpReq.Header.Set("x-canva-request", "createthread")
	httpReq.Header.Set("x-canva-app", "home")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return "", fmt.Errorf("authentication failed: cookies expired or invalid")
	}
	if resp.StatusCode == 429 {
		return "", fmt.Errorf("rate limit exceeded")
	}
	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("server error: status %d", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var threadResp struct {
		A string `json:"A"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&threadResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if threadResp.A == "" {
		return "", fmt.Errorf("no thread_id in response")
	}

	return threadResp.A, nil
}

func (p *CanvaProvider) pollThreadResults(ctx context.Context, tokenData *canvaTokenData, cookieHeader, threadID string) ([]string, error) {
	msgSeq := 0
	fragOffset := 0
	var images []string
	seenURLs := make(map[string]bool)

	for attempt := 0; attempt < maxPollAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}

		url := fmt.Sprintf("%s/%s?afterMessageSeq=%d&updateFragmentsOffset=%d&withThumbnail=true",
			canvaAPIEndpoint, threadID, msgSeq, fragOffset)

		httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}

		httpReq.Header.Set("Cookie", cookieHeader)
		httpReq.Header.Set("x-canva-authz", tokenData.CAZ)
		httpReq.Header.Set("x-canva-brand", tokenData.CB)
		httpReq.Header.Set("x-canva-user", tokenData.UserID)
		httpReq.Header.Set("x-canva-active-user", tokenData.CAU)
		httpReq.Header.Set("x-canva-accept-prefix", "no-prefix")
		httpReq.Header.Set("x-canva-request", "getthread")
		httpReq.Header.Set("x-canva-app", "home")

		resp, err := p.client.Do(httpReq)
		if err != nil {
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			continue
		}

		var pollResp struct {
			A  []map[string]interface{} `json:"A"`
			E  map[string]interface{}   `json:"e"`
			F  []map[string]interface{} `json:"f"`
			AQ string                   `json:"A?"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&pollResp); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		// Extract images from fragments
		for _, f := range pollResp.F {
			ftype, _ := f["A?"].(string)
			if ftype == "L" {
				if u, ok := f["U"].(map[string]interface{}); ok {
					if v, ok := u["V"].(map[string]interface{}); ok {
						for _, val := range v {
							if valMap, ok := val.(map[string]interface{}); ok {
								if url, ok := valMap["A"].(string); ok && url != "" && !seenURLs[url] {
									seenURLs[url] = true
									images = append(images, url)
								}
							}
						}
					}
				}
			}
		}

		fragOffset += len(pollResp.F)

		// Extract final images from messages
		for _, m := range pollResp.A {
			if mSeq, ok := m["B"].(float64); ok {
				if int(mSeq) > msgSeq {
					msgSeq = int(mSeq)
				}
			}

			if a, ok := m["A"].(map[string]interface{}); ok {
				if q, ok := a["Q"].(map[string]interface{}); ok {
					if qType, _ := q["A?"].(string); qType == "C" {
						if imgs, ok := q["Q"].([]interface{}); ok {
							var finalImages []string
							for _, img := range imgs {
								if imgMap, ok := img.(map[string]interface{}); ok {
									fullURL, _ := imgMap["K"].(string)
									thumbURL, _ := imgMap["L"].(string)

									if fullURL != "" && strings.Contains(fullURL, "image-resize") && !seenURLs[fullURL] {
										seenURLs[fullURL] = true
										finalImages = append(finalImages, fullURL)
									} else if thumbURL != "" && strings.Contains(thumbURL, "image-resize") && !seenURLs[thumbURL] {
										seenURLs[thumbURL] = true
										finalImages = append(finalImages, thumbURL)
									}
								}
							}
							if len(finalImages) > 0 {
								images = finalImages
							}
						}
					}
				}
			}
		}

		// Check if thread is complete
		if state, ok := pollResp.E["A"].(string); ok && state == "E" {
			break
		}
		if pollResp.AQ == "C" {
			break
		}

		// If we have images, continue polling a bit more to get final versions
		if len(images) > 0 && attempt > 5 {
			break
		}
	}

	return images, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *CanvaProvider) SendChatCompletionStream(ctx context.Context, account *models.Account, req *provider.ChatRequest, writer http.ResponseWriter) error {
	return fmt.Errorf("streaming not supported for image generation")
}

func (p *CanvaProvider) ValidateAccount(ctx context.Context, account *models.Account) error {
	tokenData, err := p.extractTokenData(account)
	if err != nil {
		return err
	}

	cookieHeader, err := p.buildCookieHeader(tokenData)
	if err != nil {
		return err
	}

	// Try to fetch quota as validation
	httpReq, err := http.NewRequestWithContext(ctx, "POST", canvaQuotaEndpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Cookie", cookieHeader)
	httpReq.Header.Set("Content-Type", "application/json;charset=UTF-8")
	httpReq.Header.Set("x-canva-authz", tokenData.CAZ)
	httpReq.Header.Set("x-canva-brand", tokenData.CB)
	httpReq.Header.Set("x-canva-user", tokenData.UserID)
	httpReq.Header.Set("x-canva-active-user", tokenData.CAU)
	httpReq.Header.Set("x-canva-accept-prefix", "no-prefix")
	httpReq.Header.Set("x-canva-request", "getquota")
	httpReq.Header.Set("x-canva-app", "home")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("validation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("invalid or expired cookies")
	}

	return nil
}

func (p *CanvaProvider) GetCredits(ctx context.Context, account *models.Account) (used float64, total float64, err error) {
	tokenData, err := p.extractTokenData(account)
	if err != nil {
		return 0, 0, err
	}

	cookieHeader, err := p.buildCookieHeader(tokenData)
	if err != nil {
		return 0, 0, err
	}

	quotaReq := canvaQuotaRequest{
		A: "C",
		B: tokenData.CB,
		C: tokenData.UserID,
	}

	reqBody, err := json.Marshal(quotaReq)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to marshal quota request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", canvaQuotaEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Cookie", cookieHeader)
	httpReq.Header.Set("Content-Type", "application/json;charset=UTF-8")
	httpReq.Header.Set("x-canva-authz", tokenData.CAZ)
	httpReq.Header.Set("x-canva-brand", tokenData.CB)
	httpReq.Header.Set("x-canva-user", tokenData.UserID)
	httpReq.Header.Set("x-canva-active-user", tokenData.CAU)
	httpReq.Header.Set("x-canva-accept-prefix", "no-prefix")
	httpReq.Header.Set("x-canva-request", "getquota")
	httpReq.Header.Set("x-canva-app", "home")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return 0, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 100, 100, nil // Return default values on error
	}

	var quotaResp canvaQuotaResponse
	if err := json.NewDecoder(resp.Body).Decode(&quotaResp); err != nil {
		return 100, 100, nil
	}

	usedRaw, _ := quotaResp.A["C"].(float64)
	limitRaw, _ := quotaResp.A["D"].(float64)

	if limitRaw == 0 {
		return 0, 100, nil
	}

	return usedRaw, limitRaw, nil
}
