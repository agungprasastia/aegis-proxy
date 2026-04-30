package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/aegis-proxy/aegis/internal/accounts"
	"github.com/aegis-proxy/aegis/internal/api"
	"github.com/aegis-proxy/aegis/internal/config"
	"github.com/aegis-proxy/aegis/internal/dashboard"
	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/logger"
	"github.com/aegis-proxy/aegis/internal/proxy"
	"github.com/aegis-proxy/aegis/internal/proxypool"
)

func TestProxyServerStartsOnConfiguredPort(t *testing.T) {
	cfg := testConfig(t)
	cfg.ProxyPort = freePort(t)

	db := testDB(t)
	am := accounts.NewAccountManager(db)
	rl := logger.NewRequestLogger(db)
	pool := proxypool.NewProxyPool()
	server := proxy.NewProxyServer(cfg, am, rl, pool)

	startServer(t, server.Start, func(ctx context.Context) error {
		return server.Shutdown(ctx)
	})

	assertURLAccessible(t, fmt.Sprintf("http://%s/health", cfg.ProxyAddr()))
}

func TestDashboardServerStarts(t *testing.T) {
	cfg := testConfig(t)
	cfg.DashboardPort = freePort(t)

	dashboardServer := newTestDashboardServer(t, cfg)
	startServer(t, dashboardServer.Start, func(ctx context.Context) error {
		return dashboardServer.Shutdown(ctx)
	})

	assertURLAccessible(t, fmt.Sprintf("http://%s/health", cfg.DashboardAddr()))
}

func TestProxyAndDashboardServersAccessible(t *testing.T) {
	cfg := testConfig(t)
	cfg.ProxyPort = freePort(t)
	cfg.DashboardPort = freePort(t)

	db := testDB(t)
	am := accounts.NewAccountManager(db)
	rl := logger.NewRequestLogger(db)
	pool := proxypool.NewProxyPool()
	proxyServer := proxy.NewProxyServer(cfg, am, rl, pool)
	dashboardServer := newTestDashboardServerWithDeps(cfg, db, am, rl, pool)

	startServer(t, proxyServer.Start, func(ctx context.Context) error {
		return proxyServer.Shutdown(ctx)
	})
	startServer(t, dashboardServer.Start, func(ctx context.Context) error {
		return dashboardServer.Shutdown(ctx)
	})

	assertURLAccessible(t, fmt.Sprintf("http://%s/health", cfg.ProxyAddr()))
	assertURLAccessible(t, fmt.Sprintf("http://%s/health", cfg.DashboardAddr()))
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.ProxyHost = "127.0.0.1"
	cfg.DashboardHost = "127.0.0.1"
	cfg.DataDir = t.TempDir()
	cfg.DBPath = filepath.Join(cfg.DataDir, "aegis.db")
	cfg.FilePath = filepath.Join(cfg.DataDir, "config.json")
	cfg.SetAPIKey("test-api-key")
	return cfg
}

func testDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "aegis.db"))
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newTestDashboardServer(t *testing.T, cfg *config.Config) *dashboard.DashboardServer {
	t.Helper()
	db := testDB(t)
	am := accounts.NewAccountManager(db)
	rl := logger.NewRequestLogger(db)
	pool := proxypool.NewProxyPool()
	return newTestDashboardServerWithDeps(cfg, db, am, rl, pool)
}

func newTestDashboardServerWithDeps(cfg *config.Config, db *database.DB, am *accounts.AccountManager, rl *logger.RequestLogger, pool *proxypool.ProxyPool) *dashboard.DashboardServer {
	poolConfig := &proxypool.ProxyPoolConfig{}
	apiServer := api.NewAPIServer(db, am, rl, cfg, pool, poolConfig, nil)
	return dashboard.NewDashboardServer(cfg, apiServer)
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func startServer(t *testing.T, start func() error, shutdown func(context.Context) error) {
	t.Helper()
	errCh := make(chan error, 1)
	go func() {
		if err := start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("shutdown server: %v", err)
		}
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("server error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("server did not stop")
		}
	})
}

func assertURLAccessible(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)

	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("%s not accessible: %v", url, lastErr)
}
