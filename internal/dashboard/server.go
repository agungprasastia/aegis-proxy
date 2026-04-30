package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/aegis-proxy/aegis/internal/api"
	"github.com/aegis-proxy/aegis/internal/config"
)

type DashboardServer struct {
	cfg       *config.Config
	apiServer *api.APIServer
	server    *http.Server
}

func NewDashboardServer(cfg *config.Config, apiServer *api.APIServer) *DashboardServer {
	return &DashboardServer{
		cfg:       cfg,
		apiServer: apiServer,
	}
}

func (ds *DashboardServer) Start() error {
	ds.server = &http.Server{
		Addr:    ds.cfg.DashboardAddr(),
		Handler: ds.Handler(),
	}

	fmt.Printf("Dashboard server starting on %s\n", ds.cfg.DashboardAddr())
	return ds.server.ListenAndServe()
}

func (ds *DashboardServer) Handler() http.Handler {
	mux := http.NewServeMux()

	authMiddleware := api.DashboardAuthMiddleware(ds.apiServer.Sessions)

	mux.HandleFunc("/api/auth/login", ds.apiServer.HandleLogin)
	mux.HandleFunc("/api/auth/logout", authMiddleware(http.HandlerFunc(ds.apiServer.HandleLogout)).ServeHTTP)
	mux.HandleFunc("/api/dashboard/stats", authMiddleware(http.HandlerFunc(ds.apiServer.HandleDashboardStats)).ServeHTTP)
	mux.HandleFunc("/api/accounts", ds.handleAccountsRoute(authMiddleware))
	mux.HandleFunc("/api/accounts/sync", authMiddleware(http.HandlerFunc(ds.apiServer.HandleSyncAccounts)).ServeHTTP)
	mux.HandleFunc("/api/accounts/inactive", authMiddleware(http.HandlerFunc(ds.apiServer.HandleDeleteInactiveAccounts)).ServeHTTP)
	mux.HandleFunc("/api/accounts/all", authMiddleware(http.HandlerFunc(ds.apiServer.HandleDeleteAllAccounts)).ServeHTTP)
	mux.HandleFunc("/api/models", authMiddleware(http.HandlerFunc(ds.apiServer.HandleListModels)).ServeHTTP)
	mux.HandleFunc("/api/logs", authMiddleware(http.HandlerFunc(ds.apiServer.HandleGetLogs)).ServeHTTP)
	mux.HandleFunc("/api/apikey", ds.handleAPIKeyRoute(authMiddleware))
	mux.HandleFunc("/api/apikey/regen", authMiddleware(http.HandlerFunc(ds.apiServer.HandleRegenerateAPIKey)).ServeHTTP)
	mux.HandleFunc("/api/proxies", ds.handleProxiesRoute(authMiddleware))
	mux.HandleFunc("/api/proxies/config", ds.handleProxyConfigRoute(authMiddleware))
	mux.HandleFunc("/api/proxies/test", authMiddleware(http.HandlerFunc(ds.apiServer.HandleTestProxies)).ServeHTTP)
	mux.HandleFunc("/api/settings", ds.handleSettingsRoute(authMiddleware))

	// Batch endpoints
	mux.HandleFunc("/api/batch/start", authMiddleware(http.HandlerFunc(ds.apiServer.HandleBatchStart)).ServeHTTP)
	mux.HandleFunc("/api/batch/status", authMiddleware(http.HandlerFunc(ds.apiServer.HandleBatchStatus)).ServeHTTP)
	mux.HandleFunc("/api/batch/logs", authMiddleware(http.HandlerFunc(ds.apiServer.HandleBatchLogs)).ServeHTTP)
	mux.HandleFunc("/api/batch/cancel", authMiddleware(http.HandlerFunc(ds.apiServer.HandleBatchCancel)).ServeHTTP)
	mux.HandleFunc("/api/batch/failed", authMiddleware(http.HandlerFunc(ds.apiServer.HandleBatchFailed)).ServeHTTP)
	mux.HandleFunc("/api/batch/events", ds.handleBatchEventsWithTokenAuth)

	// Account management
	mux.HandleFunc("/api/accounts/fix-errors", authMiddleware(http.HandlerFunc(ds.apiServer.HandleFixErrors)).ServeHTTP)

	// Auth
	mux.HandleFunc("/api/auth/status", ds.apiServer.HandleAuthStatus)

	// Proxy management
	mux.HandleFunc("/api/proxies/failed", authMiddleware(http.HandlerFunc(ds.apiServer.HandleDeleteFailedProxies)).ServeHTTP)
	mux.HandleFunc("/api/proxies/all", authMiddleware(http.HandlerFunc(ds.apiServer.HandleDeleteAllProxies)).ServeHTTP)

	// Filters
	mux.HandleFunc("/api/filters", ds.handleFiltersRoute(authMiddleware))
	mux.HandleFunc("/api/filters/", ds.handleFilterByIDRoute(authMiddleware))
	mux.HandleFunc("/api/filter-templates", authMiddleware(http.HandlerFunc(ds.apiServer.HandleListFilterTemplates)).ServeHTTP)

	mux.HandleFunc("/health", ds.handleHealth)
	mux.HandleFunc("/", ds.handleStatic)

	return corsMiddleware(mux)
}

func (ds *DashboardServer) Shutdown(ctx context.Context) error {
	if ds.server != nil {
		return ds.server.Shutdown(ctx)
	}
	return nil
}

func (ds *DashboardServer) handleAccountsRoute(authMiddleware func(http.Handler) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/accounts/") && r.URL.Path != "/api/accounts/" {
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleDeleteAccount)).ServeHTTP(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleListAccounts)).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleAddAccounts)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (ds *DashboardServer) handleAPIKeyRoute(authMiddleware func(http.Handler) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleGetAPIKey)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (ds *DashboardServer) handleProxiesRoute(authMiddleware func(http.Handler) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/proxies/") && r.URL.Path != "/api/proxies/" {
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleDeleteProxy)).ServeHTTP(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleListProxies)).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleAddProxy)).ServeHTTP(w, r)
		case http.MethodDelete:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleDeleteProxy)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (ds *DashboardServer) handleProxyConfigRoute(authMiddleware func(http.Handler) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleGetProxyConfig)).ServeHTTP(w, r)
		case http.MethodPut:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleUpdateProxyConfig)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (ds *DashboardServer) handleSettingsRoute(authMiddleware func(http.Handler) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleGetSettings)).ServeHTTP(w, r)
		case http.MethodPut:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleUpdateSettings)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (ds *DashboardServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	content, err := GetStaticFile(r.URL.Path)
	if err != nil {
		content, err = GetStaticFile("/index.html")
		if err != nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
	}

	contentType := "text/html"
	if strings.HasSuffix(r.URL.Path, ".js") {
		contentType = "application/javascript"
	} else if strings.HasSuffix(r.URL.Path, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(r.URL.Path, ".json") {
		contentType = "application/json"
	} else if strings.HasSuffix(r.URL.Path, ".svg") {
		contentType = "image/svg+xml"
	} else if strings.HasSuffix(r.URL.Path, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(r.URL.Path, ".ico") {
		contentType = "image/x-icon"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(content)
}

func StaticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		content, err := GetStaticFile(r.URL.Path)
		if err != nil {
			content, err = GetStaticFile("/index.html")
			if err != nil {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}
		}

		contentType := "text/html"
		if strings.HasSuffix(r.URL.Path, ".js") {
			contentType = "application/javascript"
		} else if strings.HasSuffix(r.URL.Path, ".css") {
			contentType = "text/css"
		} else if strings.HasSuffix(r.URL.Path, ".json") {
			contentType = "application/json"
		} else if strings.HasSuffix(r.URL.Path, ".svg") {
			contentType = "image/svg+xml"
		} else if strings.HasSuffix(r.URL.Path, ".png") {
			contentType = "image/png"
		} else if strings.HasSuffix(r.URL.Path, ".ico") {
			contentType = "image/x-icon"
		}

		w.Header().Set("Content-Type", contentType)
		w.Write(content)
	})
}

func (ds *DashboardServer) handleFiltersRoute(authMiddleware func(http.Handler) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleListFilters)).ServeHTTP(w, r)
		case http.MethodPost:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleAddFilter)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (ds *DashboardServer) handleFilterByIDRoute(authMiddleware func(http.Handler) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleUpdateFilter)).ServeHTTP(w, r)
		case http.MethodDelete:
			authMiddleware(http.HandlerFunc(ds.apiServer.HandleDeleteFilter)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (ds *DashboardServer) handleBatchEventsWithTokenAuth(w http.ResponseWriter, r *http.Request) {
	// SSE needs auth via query param since EventSource can't set headers
	token := r.URL.Query().Get("token")
	if token == "" {
		// Try Authorization header as fallback
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}
	}

	if token == "" || !ds.apiServer.Sessions.Validate(token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ds.apiServer.HandleBatchEvents(w, r)
}

func (ds *DashboardServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
