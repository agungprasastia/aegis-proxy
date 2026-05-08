package proxypool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProxyPoolConfig represents the configuration for the proxy pool
// This matches the enowxai proxies.json format exactly
type ProxyPoolConfig struct {
	Proxies             []ProxyEntry `json:"proxies"`
	ForKiro             bool         `json:"for_kiro"`
	ForCodeBuddy        bool         `json:"for_codebuddy"`
	ForWavespeed        bool         `json:"for_wavespeed"`
	ForCodex            bool         `json:"for_codex"`
	ForLogin            bool         `json:"for_login"`
	AutoTestEnabled     bool         `json:"auto_test_enabled"`
	AutoTestIntervalMin int          `json:"auto_test_interval_min"`
	AutoDeleteFailed    bool         `json:"auto_delete_failed"`
}

// LoadProxyConfig loads the proxy pool configuration from proxies.json
func LoadProxyConfig(dataDir string) (*ProxyPoolConfig, error) {
	configPath := filepath.Join(dataDir, "proxies.json")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return &ProxyPoolConfig{
			Proxies:             make([]ProxyEntry, 0),
			ForKiro:             true,
			ForCodeBuddy:        true,
			ForWavespeed:        false,
			ForCodex:            false,
			ForLogin:            false,
			AutoTestEnabled:     false,
			AutoTestIntervalMin: 5,
			AutoDeleteFailed:    false,
		}, nil
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read proxy config: %w", err)
	}

	// Parse JSON
	var config ProxyPoolConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse proxy config: %w", err)
	}

	// Apply global routing flags to individual proxies if not set
	for i := range config.Proxies {
		if !config.Proxies[i].ForKiro && !config.Proxies[i].ForCodeBuddy &&
			!config.Proxies[i].ForWavespeed &&
			!config.Proxies[i].ForCodex && !config.Proxies[i].ForLogin {
			// If no flags are set, use global defaults
			config.Proxies[i].ForKiro = config.ForKiro
			config.Proxies[i].ForCodeBuddy = config.ForCodeBuddy
			config.Proxies[i].ForWavespeed = config.ForWavespeed
			config.Proxies[i].ForCodex = config.ForCodex
			config.Proxies[i].ForLogin = config.ForLogin
		}
	}

	return &config, nil
}

// SaveProxyConfig saves the proxy pool configuration to proxies.json
func SaveProxyConfig(dataDir string, config *ProxyPoolConfig) error {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	configPath := filepath.Join(dataDir, "proxies.json")

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal proxy config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write proxy config: %w", err)
	}

	return nil
}

// LoadProxyPool loads proxies from config into a ProxyPool
func LoadProxyPool(dataDir string) (*ProxyPool, *ProxyPoolConfig, error) {
	config, err := LoadProxyConfig(dataDir)
	if err != nil {
		return nil, nil, err
	}

	pool := NewProxyPool()
	for _, proxy := range config.Proxies {
		pool.AddProxy(proxy)
	}

	return pool, config, nil
}

// SaveProxyPool saves a ProxyPool to config file
func SaveProxyPool(dataDir string, pool *ProxyPool, config *ProxyPoolConfig) error {
	// Update config with current pool state
	config.Proxies = pool.GetAllProxies()
	return SaveProxyConfig(dataDir, config)
}
