package router

import (
	"fmt"
	"sync"

	"github.com/aegis-proxy/aegis/internal/accounts"
	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
)

type Router struct {
	providers    map[string]provider.Provider
	accountMgr   *accounts.AccountManager
	comboService models.ComboService
	mu           sync.RWMutex
}

func NewRouter(providers map[string]provider.Provider, accountMgr *accounts.AccountManager) *Router {
	return &Router{
		providers:  providers,
		accountMgr: accountMgr,
	}
}

func (r *Router) Route(modelID string) (provider.Provider, *models.Account, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modelInfo, ok := models.GetModelInfo(modelID)
	if !ok {
		return nil, nil, fmt.Errorf("model not found: %s", modelID)
	}

	prov, exists := r.providers[modelInfo.Provider]
	if !exists {
		return nil, nil, fmt.Errorf("provider not found for model: %s", modelID)
	}

	account, err := r.accountMgr.GetAvailable(modelInfo.Provider)
	if err != nil {
		return nil, nil, fmt.Errorf("no available accounts for provider %s: %w", modelInfo.Provider, err)
	}

	return prov, account, nil
}

func (r *Router) RouteWithRetry(modelID string, maxRetries int) (provider.Provider, *models.Account, error) {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		prov, account, err := r.Route(modelID)
		if err == nil {
			return prov, account, nil
		}
		lastErr = err

		if i < maxRetries-1 {
			continue
		}
	}

	return nil, nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func (r *Router) RegisterProvider(name string, prov provider.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = prov
}

func (r *Router) GetProvider(name string) (provider.Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	prov, exists := r.providers[name]
	return prov, exists
}

func (r *Router) SetComboService(service models.ComboService) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.comboService = service
}
