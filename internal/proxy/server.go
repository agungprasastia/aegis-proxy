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
	"github.com/aegis-proxy/aegis/internal/dashboard"
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
	dashboard   http.Handler
	startTime   time.Time
	httpServer  *http.Server
	mu          sync.RWMutex
}

func (ps *ProxyServer) SetDashboardHandler(handler http.Handler) {
	ps.dashboard = handler
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

	ps.httpServer = &http.Server{
		Addr:    ps.config.ProxyAddr(),
		Handler: ps.Handler(),
	}

	fmt.Printf("Starting proxy server on %s\n", ps.config.ProxyAddr())

	if err := ps.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func (ps *ProxyServer) Handler() http.Handler {
	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/v1/chat/completions", ps.handleChatCompletions)
	proxyMux.HandleFunc("/v1/responses", ps.handleResponses)
	proxyMux.HandleFunc("/v1/messages", ps.handleMessages)
	proxyMux.HandleFunc("/v1/images/generations", ps.handleImageGeneration)
	proxyMux.HandleFunc("/v1/models", ps.handleListModels)

	proxyHandler := middleware.CORSMiddleware(
		middleware.AuthMiddleware(ps.config.GetAPIKey)(
			middleware.LoggingMiddleware(proxyMux),
		),
	)

	dashboardHandler := ps.dashboard
	if dashboardHandler == nil {
		dashboardHandler = dashboard.StaticHandler()
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/", proxyHandler)
	mux.HandleFunc("/chat", ps.handleChatUI)
	mux.HandleFunc("/health", ps.handleHealth)
	mux.Handle("/api/", dashboardHandler)
	mux.Handle("/dashboard", http.StripPrefix("/dashboard", dashboardHandler))
	mux.Handle("/dashboard/", http.StripPrefix("/dashboard", dashboardHandler))
	mux.Handle("/", dashboardHandler)

	return mux
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
