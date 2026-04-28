package proxy

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aegis-proxy/aegis/internal/accounts"
	"github.com/aegis-proxy/aegis/internal/config"
	"github.com/aegis-proxy/aegis/internal/logger"
	"github.com/aegis-proxy/aegis/internal/middleware"
	"github.com/aegis-proxy/aegis/internal/provider"
	"github.com/aegis-proxy/aegis/internal/proxypool"
	"github.com/aegis-proxy/aegis/internal/router"
)

type ProxyServer struct {
	config      *config.Config
	providers   map[string]provider.Provider
	accountMgr  *accounts.AccountManager
	logger      *logger.RequestLogger
	jsonlLogger *logger.JSONLLogger
	router      *router.Router
	pool        *proxypool.ProxyPool
	startTime   time.Time
	httpServer  *http.Server
	mu          sync.RWMutex
}

func NewProxyServer(cfg *config.Config, accountMgr *accounts.AccountManager, reqLogger *logger.RequestLogger, pool *proxypool.ProxyPool) *ProxyServer {
	// Initialize JSONL logger
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Warning: failed to get home directory: %v\n", err)
		homeDir = "."
	}
	jsonlPath := filepath.Join(homeDir, ".aegis-proxy", "request_logs.jsonl")
	jsonlLogger, err := logger.NewJSONLLogger(jsonlPath)
	if err != nil {
		fmt.Printf("Warning: failed to initialize JSONL logger: %v\n", err)
	}
	
	ps := &ProxyServer{
		config:      cfg,
		providers:   make(map[string]provider.Provider),
		accountMgr:  accountMgr,
		logger:      reqLogger,
		jsonlLogger: jsonlLogger,
		pool:        pool,
		startTime:   time.Now(),
	}
	
	ps.router = router.NewRouter(ps.providers, accountMgr)
	
	// Inject proxy pool into providers that support it
	type ProxyPoolSetter interface {
		SetProxyPool(pool *proxypool.ProxyPool)
	}
	
	for name, p := range provider.ProviderRegistry {
		ps.RegisterProvider(name, p)
		if setter, ok := p.(ProxyPoolSetter); ok {
			setter.SetProxyPool(pool)
		}
	}
	
	return ps
}

func (ps *ProxyServer) RegisterProvider(name string, p provider.Provider) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.providers[name] = p
}

func (ps *ProxyServer) Start() error {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/v1/chat/completions", ps.handleChatCompletions)
	mux.HandleFunc("/v1/responses", ps.handleResponses)
	mux.HandleFunc("/v1/messages", ps.handleMessages)
	mux.HandleFunc("/v1/images/generations", ps.handleImageGeneration)
	mux.HandleFunc("/v1/models", ps.handleListModels)
	mux.HandleFunc("/chat", ps.handleChatUI)
	mux.HandleFunc("/health", ps.handleHealth)
	handler := middleware.CORSMiddleware(
		middleware.AuthMiddleware(ps.config.GetAPIKey)(
			middleware.LoggingMiddleware(mux),
		),
	)
	
	ps.httpServer = &http.Server{
		Addr:    ps.config.ProxyAddr(),
		Handler: handler,
	}
	
	fmt.Printf("Starting proxy server on %s\n", ps.config.ProxyAddr())
	
	if err := ps.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	
	return nil
}

func (ps *ProxyServer) Shutdown(ctx context.Context) error {
	if ps.httpServer == nil {
		return nil
	}
	
	fmt.Println("Shutting down proxy server...")
	return ps.httpServer.Shutdown(ctx)
}

func (ps *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ps.httpServer != nil && ps.httpServer.Handler != nil {
		ps.httpServer.Handler.ServeHTTP(w, r)
	}
}


