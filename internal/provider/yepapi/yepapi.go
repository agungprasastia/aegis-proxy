package yepapi

import (
	"context"
	"net/http"

	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
)

type YepAPIProvider struct {
	client *http.Client
}

func init() {
	provider.ProviderRegistry["yepapi"] = &YepAPIProvider{
		client: &http.Client{},
	}
}

func (p *YepAPIProvider) Name() string {
	return "YepAPI"
}

func (p *YepAPIProvider) Tier() string {
	return "yepapi"
}

func (p *YepAPIProvider) SupportsModel(modelID string) bool {
	info, ok := models.GetModelInfo(modelID)
	if !ok {
		return false
	}
	return info.Provider == models.ProviderYepAPI
}

func (p *YepAPIProvider) MapModel(modelID string) string {
	// TODO: Map to YepAPI internal model ID if needed
	return modelID
}

func (p *YepAPIProvider) SendChatCompletion(ctx context.Context, account *models.Account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	// TODO: Implement actual API call to YepAPI
	return nil, nil
}

func (p *YepAPIProvider) SendChatCompletionStream(ctx context.Context, account *models.Account, req *provider.ChatRequest, writer http.ResponseWriter) error {
	// TODO: Implement streaming API call to YepAPI
	return nil
}

func (p *YepAPIProvider) ValidateAccount(ctx context.Context, account *models.Account) error {
	// TODO: Validate token/cookie with YepAPI
	return nil
}

func (p *YepAPIProvider) GetCredits(ctx context.Context, account *models.Account) (used float64, total float64, err error) {
	// TODO: Fetch credits from YepAPI
	return 0, 0, nil
}
