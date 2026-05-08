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
	"github.com/aegis-proxy/aegis/internal/batch"
	"github.com/aegis-proxy/aegis/internal/combo"
	"github.com/aegis-proxy/aegis/internal/config"
	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/filter"
	"github.com/aegis-proxy/aegis/internal/logger"
	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/proxypool"
	appsync "github.com/aegis-proxy/aegis/internal/sync"
	"golang.org/x/crypto/bcrypt"
)

type APIServer struct {
	db           *database.DB
	am           *accounts.AccountManager
	logger       *logger.RequestLogger
	cfg          *config.Config
	syncer       *appsync.AccountSyncer
	pool         *proxypool.ProxyPool
	poolConfig   *proxypool.ProxyPoolConfig
	tester       *proxypool.ProxyTester
	Sessions     *SessionManager
	FilterEngine *filter.FilterEngine
	BatchMgr     *batch.BatchManager
	ComboService *combo.Service
	StartTime    time.Time
}

func NewAPIServer(db *database.DB, am *accounts.AccountManager, rl *logger.RequestLogger, cfg *config.Config, pool *proxypool.ProxyPool, poolConfig *proxypool.ProxyPoolConfig, tester *proxypool.ProxyTester) *APIServer {
	// Initialize filter engine with default filters
	filterEngine, err := filter.NewFilterEngine(filter.GetDefaultFilters())
	if err != nil {
		filterEngine, _ = filter.NewFilterEngine(nil)
	}

	return &APIServer{
		db:           db,
		am:           am,
		logger:       rl,
		cfg:          cfg,
		syncer:       appsync.NewAccountSyncer(db),
		pool:         pool,
		poolConfig:   poolConfig,
		tester:       tester,
		Sessions:     NewSessionManager(),
		FilterEngine: filterEngine,
		BatchMgr:     batch.NewBatchManager(am),
		ComboService: combo.NewService(db),
		StartTime:    time.Now(),
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

	// Get request stats from logger
	var totalRequests, successRequests int
	var successRate float64
	if s.logger != nil {
		logs, _, _ := s.logger.GetLogs(1, 10000, nil)
		totalRequests = len(logs)
		for _, l := range logs {
			if l.Status == "success" || l.StatusCode == 200 {
				successRequests++
			}
		}
		if totalRequests > 0 {
			successRate = float64(successRequests) / float64(totalRequests) * 100
		}
	}

	// Calculate uptime
	uptime := time.Since(s.StartTime)
	uptimeStr := formatUptime(uptime)

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
		"requests": map[string]interface{}{
			"total":        totalRequests,
			"success":      successRequests,
			"success_rate": successRate,
		},
		"uptime":         uptimeStr,
		"uptime_seconds": int(uptime.Seconds()),
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

	// Return empty array instead of null
	if accounts == nil {
		accounts = []*models.Account{}
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

	// Return empty array instead of null
	if logs == nil {
		logs = []models.RequestLog{}
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
		respondJSON(w, http.StatusOK, map[string]string{"api_key": "", "api_key_full": ""})
		return
	}

	masked := maskAPIKey(apiKey)
	respondJSON(w, http.StatusOK, map[string]string{
		"api_key":      masked,
		"api_key_full": apiKey,
	})
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
	for i := range proxies {
		proxypool.NormalizeEntry(&proxies[i])
	}
	respondJSON(w, http.StatusOK, proxies)
}

func (s *APIServer) HandleGetProxyConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"for_kiro":               s.poolConfig.ForKiro,
		"for_codebuddy":          s.poolConfig.ForCodeBuddy,
		"for_wavespeed":          s.poolConfig.ForWavespeed,
		"for_codex":              s.poolConfig.ForCodex,
		"for_login":              s.poolConfig.ForLogin,
		"auto_test_enabled":      s.poolConfig.AutoTestEnabled,
		"auto_test_interval_min": s.poolConfig.AutoTestIntervalMin,
		"auto_delete_failed":     s.poolConfig.AutoDeleteFailed,
	})
}

func (s *APIServer) HandleUpdateProxyConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ForKiro             *bool `json:"for_kiro"`
		ForCodeBuddy        *bool `json:"for_codebuddy"`
		ForWavespeed        *bool `json:"for_wavespeed"`
		ForCodex            *bool `json:"for_codex"`
		ForLogin            *bool `json:"for_login"`
		AutoTestEnabled     *bool `json:"auto_test_enabled"`
		AutoTestIntervalMin *int  `json:"auto_test_interval_min"`
		AutoDeleteFailed    *bool `json:"auto_delete_failed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.ForKiro != nil {
		s.poolConfig.ForKiro = *req.ForKiro
	}
	if req.ForCodeBuddy != nil {
		s.poolConfig.ForCodeBuddy = *req.ForCodeBuddy
	}
	if req.ForWavespeed != nil {
		s.poolConfig.ForWavespeed = *req.ForWavespeed
	}
	if req.ForCodex != nil {
		s.poolConfig.ForCodex = *req.ForCodex
	}
	if req.ForLogin != nil {
		s.poolConfig.ForLogin = *req.ForLogin
	}
	if req.AutoTestEnabled != nil {
		s.poolConfig.AutoTestEnabled = *req.AutoTestEnabled
	}
	if req.AutoTestIntervalMin != nil && *req.AutoTestIntervalMin > 0 {
		s.poolConfig.AutoTestIntervalMin = *req.AutoTestIntervalMin
	}
	if req.AutoDeleteFailed != nil {
		s.poolConfig.AutoDeleteFailed = *req.AutoDeleteFailed
	}

	s.pool.ApplyRoutingFlags(
		s.poolConfig.ForKiro,
		s.poolConfig.ForCodeBuddy,
		s.poolConfig.ForWavespeed,
		s.poolConfig.ForCodex,
		s.poolConfig.ForLogin,
	)

	if s.tester != nil {
		s.tester.UpdateConfig(s.poolConfig)
	}

	if err := proxypool.SaveProxyPool(s.cfg.DataDir, s.pool, s.poolConfig); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	s.HandleGetProxyConfig(w, r)
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
		ForCodex:     s.poolConfig.ForCodex,
		ForLogin:     s.poolConfig.ForLogin,
	}
	proxypool.NormalizeEntry(&entry)

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

	proxyURL := ""

	// Try to get URL from request body first
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.URL != "" {
		proxyURL = req.URL
	}

	// Try query param
	if proxyURL == "" {
		proxyURL = r.URL.Query().Get("url")
	}

	// Try URL path: /api/proxies/{encoded_url}
	if proxyURL == "" {
		pathPart := strings.TrimPrefix(r.URL.Path, "/api/proxies/")
		if pathPart != "" && pathPart != r.URL.Path {
			proxyURL = pathPart
		}
	}

	if proxyURL == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	s.pool.RemoveProxy(proxyURL)
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
		"proxy_host":             s.cfg.ProxyHost,
		"proxy_port":             s.cfg.ProxyPort,
		"dashboard_host":         s.cfg.DashboardHost,
		"dashboard_port":         s.cfg.DashboardPort,
		"upstream_proxy":         s.cfg.UpstreamProxy,
		"account_add_headless":   s.cfg.AccountAddHeadless,
		"account_add_concurrent": s.cfg.AccountAddConcurrent,
		"account_add_priority":   s.cfg.AccountAddPriority,
		"account_add_parallel":   s.cfg.AccountAddParallel,
		"expose_to_network":      s.cfg.ExposeToNetwork,
		"whitelist_enabled":      s.cfg.WhitelistEnabled,
		"whitelisted_ips":        s.cfg.WhitelistedIPs,
		"rtk_enabled":            s.cfg.RTKEnabled,
		"sync_endpoint":          s.cfg.SyncEndpoint,
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

	// Apply settings from request body to config
	if v, ok := req["proxy_host"].(string); ok && v != "" {
		s.cfg.ProxyHost = v
	}
	if v, ok := req["proxy_port"].(float64); ok {
		s.cfg.ProxyPort = int(v)
	}
	if v, ok := req["dashboard_host"].(string); ok && v != "" {
		s.cfg.DashboardHost = v
	}
	if v, ok := req["dashboard_port"].(float64); ok {
		s.cfg.DashboardPort = int(v)
	}
	if v, ok := req["expose_to_network"].(bool); ok {
		s.cfg.ExposeToNetwork = v
	}
	if v, ok := req["upstream_proxy"].(string); ok {
		s.cfg.UpstreamProxy = v
	}
	if v, ok := req["rtk_enabled"].(bool); ok {
		s.cfg.RTKEnabled = v
	}
	if v, ok := req["sync_endpoint"].(string); ok {
		s.cfg.SyncEndpoint = v
	}
	if v, ok := req["account_add_concurrent"].(float64); ok {
		s.cfg.AccountAddConcurrent = int(v)
	}
	if v, ok := req["account_add_priority"].(string); ok {
		s.cfg.AccountAddPriority = v
	}
	if v, ok := req["account_add_parallel"].(float64); ok {
		s.cfg.AccountAddParallel = int(v)
	}
	if v, ok := req["account_add_headless"].(bool); ok {
		s.cfg.AccountAddHeadless = v
	}
	if v, ok := req["whitelisted_ips"].([]interface{}); ok {
		ips := make([]string, 0, len(v))
		for _, ip := range v {
			if s, ok := ip.(string); ok && s != "" {
				ips = append(ips, s)
			}
		}
		s.cfg.WhitelistedIPs = ips
		s.cfg.WhitelistEnabled = len(ips) > 0
	}

	// Handle password change
	if newPassword, ok := req["dashboard_password"].(string); ok && newPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
			return
		}
		s.cfg.DashboardPasswordHash = string(hash)
		s.cfg.DashboardPassword = "" // Clear plaintext
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

// ==================== Additional Handlers ====================

func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func (s *APIServer) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractToken(r)
	if token != "" && s.Sessions.Validate(token) {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"authenticated": true,
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": false,
	})
}

func (s *APIServer) HandleBatchFailed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get accounts with error status
	allAccounts, err := s.am.GetAll("")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	failed := make([]*models.Account, 0)
	for _, acc := range allAccounts {
		if acc.Status == models.StatusError || acc.Status == models.StatusBanned {
			failed = append(failed, acc)
		}
	}

	respondJSON(w, http.StatusOK, failed)
}

func (s *APIServer) HandleFixErrors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all error accounts
	allAccounts, err := s.am.GetAll("")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	errorAccounts := make([]batch.AccountInput, 0)
	for _, acc := range allAccounts {
		if acc.Status == models.StatusError {
			errorAccounts = append(errorAccounts, batch.AccountInput{
				Email:    acc.Email,
				Password: acc.Password,
			})
		}
	}

	if len(errorAccounts) == 0 {
		respondJSON(w, http.StatusOK, map[string]string{"message": "No error accounts to fix"})
		return
	}

	// Start batch re-login for error accounts
	cfg := batch.BatchConfig{
		Concurrent: 1,
		Headless:   s.cfg.AccountAddHeadless,
		Priority:   "standard",
		ProxyURL:   s.cfg.UpstreamProxy,
	}

	if err := s.BatchMgr.Start(errorAccounts, cfg); err != nil {
		respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Re-logging %d error accounts", len(errorAccounts)),
		"count":   len(errorAccounts),
	})
}

func (s *APIServer) HandleDeleteFailedProxies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	proxies := s.pool.GetAllProxies()
	count := 0
	for _, p := range proxies {
		if p.Status == "failed" {
			s.pool.RemoveProxy(p.URL)
			count++
		}
	}

	proxypool.SaveProxyPool(s.cfg.DataDir, s.pool, s.poolConfig)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Deleted %d failed proxies", count),
		"count":   count,
	})
}

func (s *APIServer) HandleDeleteAllProxies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	proxies := s.pool.GetAllProxies()
	for _, p := range proxies {
		s.pool.RemoveProxy(p.URL)
	}

	proxypool.SaveProxyPool(s.cfg.DataDir, s.pool, s.poolConfig)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Deleted %d proxies", len(proxies)),
		"count":   len(proxies),
	})
}

func (s *APIServer) HandleListFilterTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	templates := map[string]interface{}{
		"basic": map[string]interface{}{
			"name":        "Basic",
			"description": "9 essential obfuscation rules",
			"count":       9,
		},
		"aggressive": map[string]interface{}{
			"name":        "Aggressive",
			"description": "31 rules with full obfuscation (default)",
			"count":       31,
		},
		"minimal": map[string]interface{}{
			"name":        "Minimal",
			"description": "3 rules for light filtering",
			"count":       3,
		},
	}

	respondJSON(w, http.StatusOK, templates)
}

// ==================== SSE Handler ====================

func (s *APIServer) HandleBatchEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ctx := r.Context()
	lastLogCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		status := s.BatchMgr.GetStatus()

		// Send status update
		statusJSON, _ := json.Marshal(status)
		fmt.Fprintf(w, "event: status\ndata: %s\n\n", statusJSON)

		// Send new logs only
		if len(status.Logs) > lastLogCount {
			newLogs := status.Logs[lastLogCount:]
			for _, log := range newLogs {
				logJSON, _ := json.Marshal(log)
				fmt.Fprintf(w, "event: log\ndata: %s\n\n", logJSON)
			}
			lastLogCount = len(status.Logs)
		}

		// Send account-added event when success count changes
		if !status.Active && status.Status == "completed" {
			completeJSON, _ := json.Marshal(map[string]interface{}{
				"success": status.Success,
				"failed":  status.Failed,
				"total":   status.Total,
			})
			fmt.Fprintf(w, "event: complete\ndata: %s\n\n", completeJSON)
			flusher.Flush()
			return
		}

		flusher.Flush()
		time.Sleep(500 * time.Millisecond)
	}
}

// ==================== Batch Handlers ====================

func (s *APIServer) HandleBatchStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Accounts []batch.AccountInput `json:"accounts"`
		Provider string               `json:"provider"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if len(req.Accounts) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "No accounts provided"})
		return
	}

	concurrent := s.cfg.AccountAddConcurrent
	if concurrent < 1 {
		concurrent = 1
	}
	priority := s.cfg.AccountAddPriority
	if priority == "" {
		priority = "standard"
	}

	cfg := batch.BatchConfig{
		Concurrent: concurrent,
		Headless:   s.cfg.AccountAddHeadless,
		Priority:   priority,
		Provider:   req.Provider,
		ProxyURL:   s.cfg.UpstreamProxy,
	}

	if err := s.BatchMgr.Start(req.Accounts, cfg); err != nil {
		respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *APIServer) HandleBatchStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.BatchMgr.GetStatus()
	respondJSON(w, http.StatusOK, status)
}

func (s *APIServer) HandleBatchLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logs := s.BatchMgr.GetLogs()
	respondJSON(w, http.StatusOK, logs)
}

func (s *APIServer) HandleBatchCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.BatchMgr.Cancel()
	respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// ==================== Filter Handlers ====================

type filterResponse struct {
	ID          string `json:"id"`
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
	Mode        string `json:"mode"`
	IsRegex     bool   `json:"is_regex"`
	Active      bool   `json:"active"`
}

func filterRuleToResponse(r filter.FilterRule) filterResponse {
	mode := "string"
	if r.IsRegex {
		mode = "regex"
	}
	return filterResponse{
		ID:          r.ID,
		Pattern:     r.Pattern,
		Replacement: r.Replacement,
		Mode:        mode,
		IsRegex:     r.IsRegex,
		Active:      r.IsActive,
	}
}

func (s *APIServer) HandleListFilters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filters := s.FilterEngine.ListFilters()
	result := make([]filterResponse, 0, len(filters))
	for _, f := range filters {
		result = append(result, filterRuleToResponse(f))
	}

	respondJSON(w, http.StatusOK, result)
}

func (s *APIServer) HandleAddFilter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Pattern     string `json:"pattern"`
		Replacement string `json:"replacement"`
		Mode        string `json:"mode"`
		Active      bool   `json:"active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Pattern == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Pattern is required"})
		return
	}

	rule := filter.FilterRule{
		Pattern:     req.Pattern,
		Replacement: req.Replacement,
		IsRegex:     req.Mode == "regex",
		IsActive:    req.Active,
		Mode:        filter.FilterModeBoth,
	}

	if err := s.FilterEngine.AddFilter(rule); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *APIServer) HandleUpdateFilter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filter ID from path: /api/filters/{id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/filters/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Filter ID required"})
		return
	}
	filterID := parts[0]

	var req struct {
		Active *bool `json:"active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Active != nil {
		if err := s.FilterEngine.SetActive(filterID, *req.Active); err != nil {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *APIServer) HandleDeleteFilter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filter ID from path: /api/filters/{id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/filters/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Filter ID required"})
		return
	}
	filterID := parts[0]

	if err := s.FilterEngine.RemoveFilter(filterID); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *APIServer) HandleListCombos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	combos, err := s.ComboService.List()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, combos)
}

func (s *APIServer) HandleAddCombo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.Combo
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	if err := s.ComboService.Create(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, req)
}

func (s *APIServer) HandleUpdateCombo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, ok := comboIDFromPath(w, r)
	if !ok {
		return
	}
	var req models.Combo
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	req.ID = id
	if err := s.ComboService.Update(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, req)
}

func (s *APIServer) HandleDeleteCombo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, ok := comboIDFromPath(w, r)
	if !ok {
		return
	}
	if err := s.ComboService.Delete(id); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func comboIDFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/combos/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Combo ID required"})
		return 0, false
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid combo ID"})
		return 0, false
	}
	return id, true
}
