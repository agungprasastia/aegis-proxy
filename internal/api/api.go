package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aegis-proxy/aegis/internal/accounts"
	"github.com/aegis-proxy/aegis/internal/auth"
	"github.com/aegis-proxy/aegis/internal/config"
	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/logger"
	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/proxypool"
	appsync "github.com/aegis-proxy/aegis/internal/sync"
	"golang.org/x/crypto/bcrypt"
)

type APIServer struct {
	db         *database.DB
	am         *accounts.AccountManager
	logger     *logger.RequestLogger
	cfg        *config.Config
	syncer     *appsync.AccountSyncer
	pool       *proxypool.ProxyPool
	poolConfig *proxypool.ProxyPoolConfig
	Sessions   *SessionManager
}

func NewAPIServer(db *database.DB, am *accounts.AccountManager, rl *logger.RequestLogger, cfg *config.Config, pool *proxypool.ProxyPool, poolConfig *proxypool.ProxyPoolConfig) *APIServer {
	return &APIServer{
		db:         db,
		am:         am,
		logger:     rl,
		cfg:        cfg,
		syncer:     appsync.NewAccountSyncer(db),
		pool:       pool,
		poolConfig: poolConfig,
		Sessions:   NewSessionManager(),
	}
}

func (s *APIServer) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	// Check if password hash exists
	if s.cfg.DashboardPasswordHash == "" {
		// Backward compatibility: if no hash but plaintext exists, compare plaintext
		if s.cfg.DashboardPassword != "" && req.Password == s.cfg.DashboardPassword {
			token := s.Sessions.Create()
			respondJSON(w, http.StatusOK, map[string]string{"token": token})
			return
		}
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid password"})
		return
	}

	// Compare with bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(s.cfg.DashboardPasswordHash), []byte(req.Password)); err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid password"})
		return
	}

	token := s.Sessions.Create()
	respondJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *APIServer) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractToken(r)
	if token != "" {
		s.Sessions.Delete(token)
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Logged out"})
}

func (s *APIServer) HandleDashboardStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	accounts, err := s.am.GetAll("")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var active, exhausted, banned, errorCount int
	var totalCredits, usedCredits float64
	var lastSync *time.Time
	for _, acc := range accounts {
		totalCredits += acc.CreditsTotal
		usedCredits += acc.CreditsUsed
		if acc.LastSyncedAt != nil && (lastSync == nil || acc.LastSyncedAt.After(*lastSync)) {
			lastSync = acc.LastSyncedAt
		}
		switch acc.Status {
		case models.StatusActive:
			active++
		case models.StatusExhausted:
			exhausted++
		case models.StatusBanned:
			banned++
		case models.StatusError:
			errorCount++
		}
	}

	response := map[string]interface{}{
		"accounts": map[string]interface{}{
			"total":     len(accounts),
			"active":    active,
			"exhausted": exhausted,
			"banned":    banned,
			"error":     errorCount,
		},
		"credits": map[string]interface{}{
			"total":     totalCredits,
			"used":      usedCredits,
			"remaining": totalCredits - usedCredits,
		},
	}
	if lastSync != nil {
		response["lastSync"] = lastSync.UTC().Format(time.RFC3339)
	}
	respondJSON(w, http.StatusOK, response)
}

func (s *APIServer) HandleListAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := r.URL.Query().Get("provider")
	accounts, err := s.am.GetAll(provider)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, accounts)
}

func (s *APIServer) HandleAddAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Provider string `json:"provider"`
		Accounts string `json:"accounts"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Provider == "" || req.Accounts == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Provider and accounts are required"})
		return
	}

	count, err := s.am.BatchAdd(req.Provider, req.Accounts)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Added %d accounts", count),
		"count":   count,
	})
}

func (s *APIServer) HandleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/accounts/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid account ID"})
		return
	}

	if err := s.am.Remove(id); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Account deleted"})
}

func (s *APIServer) HandleSyncAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.syncer == nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "syncer unavailable"})
		return
	}

	if err := s.syncer.Sync(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status":    "syncing",
		"message":   "Account sync started",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *APIServer) HandleDeleteInactiveAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	count, err := s.am.DeleteInactive()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Deleted %d inactive accounts", count),
		"count":   count,
	})
}

func (s *APIServer) HandleDeleteAllAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	count, err := s.am.DeleteAll()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Deleted %d accounts", count),
		"count":   count,
	})
}

func (s *APIServer) HandleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allModels := models.ListAllModels()

	grouped := make(map[string][]models.ModelInfo)
	for _, model := range allModels {
		grouped[model.Tier] = append(grouped[model.Tier], model)
	}

	respondJSON(w, http.StatusOK, grouped)
}

func (s *APIServer) HandleGetLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 50
	}

	filters := make(map[string]string)
	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = status
	}
	if model := r.URL.Query().Get("model"); model != "" {
		filters["model"] = model
	}
	if provider := r.URL.Query().Get("provider"); provider != "" {
		filters["provider"] = provider
	}

	logs, total, err := s.logger.GetLogs(page, limit, filters)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (s *APIServer) HandleGetAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apiKey := s.cfg.GetAPIKey()
	if apiKey == "" {
		respondJSON(w, http.StatusOK, map[string]string{"api_key": ""})
		return
	}

	masked := maskAPIKey(apiKey)
	respondJSON(w, http.StatusOK, map[string]string{"api_key": masked})
}

func (s *APIServer) HandleRegenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	newKey := auth.GenerateAPIKey()
	s.cfg.SetAPIKey(newKey)

	if err := s.cfg.Save(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"api_key": newKey,
		"message": "API key regenerated",
	})
}

func (s *APIServer) HandleListProxies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	proxies := s.pool.GetAllProxies()
	respondJSON(w, http.StatusOK, proxies)
}

func (s *APIServer) HandleAddProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL    string `json:"url"`
		Type   string `json:"type"`
		Region string `json:"region"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.URL == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	entry := proxypool.ProxyEntry{
		URL:          req.URL,
		Type:         req.Type,
		Region:       req.Region,
		Status:       "ok",
		ForKiro:      s.poolConfig.ForKiro,
		ForCodeBuddy: s.poolConfig.ForCodeBuddy,
		ForWavespeed: s.poolConfig.ForWavespeed,
		ForYepAPI:    s.poolConfig.ForYepAPI,
		ForCodex:     s.poolConfig.ForCodex,
		ForLogin:     s.poolConfig.ForLogin,
	}

	s.pool.AddProxy(entry)
	proxypool.SaveProxyPool(s.cfg.DataDir, s.pool, s.poolConfig)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Proxy added",
		"proxy":   entry,
	})
}

func (s *APIServer) HandleDeleteProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.URL = r.URL.Query().Get("url")
	} else if req.URL == "" {
		req.URL = r.URL.Query().Get("url")
	}

	if req.URL == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	s.pool.RemoveProxy(req.URL)
	proxypool.SaveProxyPool(s.cfg.DataDir, s.pool, s.poolConfig)

	respondJSON(w, http.StatusOK, map[string]string{"message": "Proxy deleted"})
}

func (s *APIServer) HandleTestProxies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	proxies := s.pool.GetAllProxies()
	results := make([]map[string]interface{}, len(proxies))

	for i, proxy := range proxies {
		err := proxypool.TestProxy(&proxy)
		status := "ok"
		errMsg := ""
		if err != nil {
			status = "failed"
			errMsg = err.Error()
		}

		s.pool.UpdateProxy(proxy.URL, status, proxy.LatencyMs)

		results[i] = map[string]interface{}{
			"url":        proxy.URL,
			"status":     status,
			"latency_ms": proxy.LatencyMs,
			"error":      errMsg,
		}
	}
	
	proxypool.SaveProxyPool(s.cfg.DataDir, s.pool, s.poolConfig)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Proxy test completed",
		"results": results,
	})
}

func (s *APIServer) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings := map[string]interface{}{
		"proxy_host":            s.cfg.ProxyHost,
		"proxy_port":            s.cfg.ProxyPort,
		"dashboard_host":        s.cfg.DashboardHost,
		"dashboard_port":        s.cfg.DashboardPort,
		"upstream_proxy":        s.cfg.UpstreamProxy,
		"account_add_headless":  s.cfg.AccountAddHeadless,
		"expose_to_network":     s.cfg.ExposeToNetwork,
		"whitelist_enabled":     s.cfg.WhitelistEnabled,
		"whitelisted_ips":       s.cfg.WhitelistedIPs,
	}

	respondJSON(w, http.StatusOK, settings)
}

func (s *APIServer) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if err := s.cfg.Save(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Settings updated"})
}



func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func extractToken(r *http.Request) string {
	if cookie, err := r.Cookie("session_token"); err == nil {
		return cookie.Value
	}

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return ""
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
