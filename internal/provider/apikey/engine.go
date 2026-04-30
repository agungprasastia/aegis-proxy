package apikey

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
)

type NormalizedRequest = models.NormalizedRequest
type NormalizedResponse = models.NormalizedResponse

type APIKeyProvider struct {
	Name       string
	BaseURL    string
	AuthHeader string
	APIPrefix  string
	Models     []string
	client     *http.Client
}

func NewAPIKeyProvider(name, baseURL, authHeader string) *APIKeyProvider {
	if authHeader == "" {
		authHeader = "Authorization"
	}
	return &APIKeyProvider{
		Name:       name,
		BaseURL:    strings.TrimRight(baseURL, "/"),
		AuthHeader: authHeader,
		APIPrefix:  "/v1",
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *APIKeyProvider) SetAPIPrefix(prefix string) {
	if prefix == "" {
		p.APIPrefix = ""
		return
	}
	p.APIPrefix = "/" + strings.Trim(prefix, "/")
}

func (p *APIKeyProvider) TestConnectivity(ctx context.Context, apiKey string) error {
	_, err := p.listModels(ctx, apiKey, true)
	return err
}

func (p *APIKeyProvider) ListModels(ctx context.Context, apiKey string) ([]string, error) {
	return p.listModels(ctx, apiKey, false)
}

func (p *APIKeyProvider) listModels(ctx context.Context, apiKey string, forceRefresh bool) ([]string, error) {
	if !forceRefresh && len(p.Models) > 0 {
		return append([]string(nil), p.Models...), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint("/models"), nil)
	if err != nil {
		return nil, err
	}
	p.setAuth(req, apiKey)

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("authentication failed: invalid or expired api_key")
		}
		return nil, fmt.Errorf("%s models failed: status %d: %s", p.Name, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID != "" {
			models = append(models, item.ID)
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("%s returned no models", p.Name)
	}
	p.Models = append([]string(nil), models...)
	return models, nil
}

func (p *APIKeyProvider) SendRequest(ctx context.Context, apiKey string, req *NormalizedRequest) (*NormalizedResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.setAuth(httpReq, apiKey)

	resp, err := p.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("authentication failed: invalid or expired api_key")
		}
		return nil, fmt.Errorf("%s request failed: status %d: %s", p.Name, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content   string                      `json:"content"`
				ToolCalls []models.NormalizedToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason models.FinishReason `json:"finish_reason"`
		} `json:"choices"`
		Usage models.NormalizedUsage `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Choices) == 0 {
		return nil, fmt.Errorf("%s returned no choices", p.Name)
	}

	choice := payload.Choices[0]
	return &models.NormalizedResponse{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Usage:        payload.Usage,
		ToolCalls:    choice.Message.ToolCalls,
	}, nil
}

func (p *APIKeyProvider) setAuth(req *http.Request, apiKey string) {
	if strings.EqualFold(p.AuthHeader, "Authorization") {
		req.Header.Set(p.AuthHeader, "Bearer "+apiKey)
		return
	}
	req.Header.Set(p.AuthHeader, apiKey)
}

func (p *APIKeyProvider) httpClient() *http.Client {
	if p.client != nil {
		return p.client
	}
	return http.DefaultClient
}

func (p *APIKeyProvider) endpoint(path string) string {
	return p.BaseURL + p.APIPrefix + path
}
