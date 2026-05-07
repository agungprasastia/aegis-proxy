package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	mu                    sync.RWMutex
	ProxyHost             string       `json:"proxy_host"`
	ProxyPort             int          `json:"proxy_port"`
	DashboardHost         string       `json:"dashboard_host"`
	DashboardPort         int          `json:"dashboard_port"`
	APIKey                string       `json:"api_key"`
	DashboardPassword     string       `json:"dashboard_password"`
	DashboardPasswordHash string       `json:"dashboard_password_hash"`
	LicenseKey            string       `json:"license_key"`
	DBPath                string       `json:"db_path"`
	UpstreamProxy         string       `json:"upstream_proxy"`
	AccountAddHeadless    bool         `json:"account_add_headless"`
	AccountAddConcurrent  int          `json:"account_add_concurrent"`
	AccountAddPriority    string       `json:"account_add_priority"`
	AccountAddParallel    int          `json:"account_add_parallel"`
	ExposeToNetwork       bool         `json:"expose_to_network"`
	WhitelistEnabled      bool         `json:"whitelist_enabled"`
	WhitelistedIPs        []string     `json:"whitelisted_ips"`
	DataDir               string       `json:"data_dir"`
	FilePath              string       `json:"-"`
	FilterMode            string       `json:"filter_mode"`
	LocalFilters          []FilterRule `json:"local_filters"`
	FilterTemplates       []string     `json:"filter_templates"`
	RTKEnabled            bool         `json:"rtk_enabled"`
	SyncEndpoint          string       `json:"sync_endpoint"`
}

// FilterRule represents a filter rule for config persistence
type FilterRule struct {
	ID            string `json:"id"`
	Pattern       string `json:"pattern"`
	Replacement   string `json:"replacement"`
	IsRegex       bool   `json:"is_regex"`
	CaseSensitive bool   `json:"case_sensitive"`
	IsActive      bool   `json:"is_active"`
	Mode          string `json:"mode"`
}

func DefaultConfig() *Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	dataDir := filepath.Join(homeDir, ".aegis-proxy")

	return &Config{
		ProxyHost:          "127.0.0.1",
		ProxyPort:          3130,
		DashboardHost:      "127.0.0.1",
		DashboardPort:      3131,
		APIKey:             "",
		DashboardPassword:  "",
		LicenseKey:         "",
		DBPath:             filepath.Join(dataDir, "aegis.db"),
		UpstreamProxy:      "",
		AccountAddHeadless: false,
		ExposeToNetwork:    false,
		WhitelistEnabled:   false,
		WhitelistedIPs:     []string{},
		DataDir:            dataDir,
		FilePath:           filepath.Join(dataDir, "config.json"),
		FilterMode:         "aggressive",
		LocalFilters:       []FilterRule{},
		FilterTemplates:    []string{"aggressive"},
		RTKEnabled:         true,
		SyncEndpoint:       "",
	}
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	data, err := os.ReadFile(cfg.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if proxyHost := os.Getenv("AEGIS_PROXY_HOST"); proxyHost != "" {
		cfg.ProxyHost = proxyHost
	}
	if dashboardHost := os.Getenv("AEGIS_DASHBOARD_HOST"); dashboardHost != "" {
		cfg.DashboardHost = dashboardHost
	}

	// Auto-migration: hash plaintext password if exists
	if cfg.DashboardPassword != "" && cfg.DashboardPasswordHash == "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.DashboardPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		cfg.DashboardPasswordHash = string(hash)
		cfg.DashboardPassword = "" // Clear plaintext for security
		if err := cfg.Save(); err != nil {
			return nil, fmt.Errorf("failed to save migrated config: %w", err)
		}
	}

	return cfg, nil
}

func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(c.FilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *Config) SetAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.APIKey = key
}

func (c *Config) GetAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.APIKey
}

func (c *Config) ProxyAddr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return fmt.Sprintf("%s:%d", c.ProxyHost, c.ProxyPort)
}

func (c *Config) DashboardAddr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return fmt.Sprintf("%s:%d", c.DashboardHost, c.DashboardPort)
}

// GetFilterMode returns the current filter mode
func (c *Config) GetFilterMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.FilterMode
}

// SetFilterMode sets the filter mode
func (c *Config) SetFilterMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.FilterMode = mode
}

// GetLocalFilters returns the local filter rules
func (c *Config) GetLocalFilters() []FilterRule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LocalFilters
}

// SetLocalFilters sets the local filter rules
func (c *Config) SetLocalFilters(filters []FilterRule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LocalFilters = filters
}

// GetFilterTemplates returns the active filter templates
func (c *Config) GetFilterTemplates() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.FilterTemplates
}

// SetFilterTemplates sets the active filter templates
func (c *Config) SetFilterTemplates(templates []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.FilterTemplates = templates
}
