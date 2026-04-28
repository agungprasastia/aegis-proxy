package proxypool

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ProxyEntry represents a single proxy in the pool
type ProxyEntry struct {
	URL          string    `json:"url"`
	Type         string    `json:"type"`           // "http" or "socks5"
	Host         string    `json:"host"`
	Port         string    `json:"port"`
	Region       string    `json:"region"`
	Status       string    `json:"status"`         // "ok" or "failed"
	LatencyMs    int       `json:"latency_ms"`
	LastChecked  time.Time `json:"last_checked"`
	ForKiro      bool      `json:"for_kiro"`
	ForCodeBuddy bool      `json:"for_codebuddy"`
	ForWavespeed bool      `json:"for_wavespeed"`
	ForYepAPI    bool      `json:"for_yepapi"`
	ForCodex     bool      `json:"for_codex"`
	ForLogin     bool      `json:"for_login"`
}

// ProxyPool manages a collection of proxies
type ProxyPool struct {
	mu      sync.RWMutex
	proxies []ProxyEntry
}

// NewProxyPool creates a new proxy pool
func NewProxyPool() *ProxyPool {
	return &ProxyPool{
		proxies: make([]ProxyEntry, 0),
	}
}

// GetProxy returns the best proxy for a given provider
// Returns nil if no suitable proxy is found
func (p *ProxyPool) GetProxy(provider string) *ProxyEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var bestProxy *ProxyEntry
	bestLatency := int(^uint(0) >> 1) // max int

	for i := range p.proxies {
		proxy := &p.proxies[i]
		
		// Skip failed proxies
		if proxy.Status != "ok" {
			continue
		}

		// Check if proxy is enabled for this provider
		if !p.isProxyEnabledForProvider(proxy, provider) {
			continue
		}

		// Select proxy with lowest latency
		if proxy.LatencyMs < bestLatency {
			bestLatency = proxy.LatencyMs
			bestProxy = proxy
		}
	}

	return bestProxy
}

// isProxyEnabledForProvider checks if a proxy is enabled for a specific provider
func (p *ProxyPool) isProxyEnabledForProvider(proxy *ProxyEntry, provider string) bool {
	switch provider {
	case "kiro":
		return proxy.ForKiro
	case "codebuddy":
		return proxy.ForCodeBuddy
	case "wavespeed":
		return proxy.ForWavespeed
	case "yepapi":
		return proxy.ForYepAPI
	case "codex":
		return proxy.ForCodex
	case "login":
		return proxy.ForLogin
	default:
		return false
	}
}

// AddProxy adds a new proxy to the pool
func (p *ProxyPool) AddProxy(entry ProxyEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if proxy already exists
	for i, existing := range p.proxies {
		if existing.URL == entry.URL {
			// Update existing proxy
			p.proxies[i] = entry
			return
		}
	}

	// Add new proxy
	p.proxies = append(p.proxies, entry)
}

// RemoveProxy removes a proxy from the pool by URL
func (p *ProxyPool) RemoveProxy(proxyURL string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, proxy := range p.proxies {
		if proxy.URL == proxyURL {
			p.proxies = append(p.proxies[:i], p.proxies[i+1:]...)
			return
		}
	}
}

// GetAllProxies returns a copy of all proxies
func (p *ProxyPool) GetAllProxies() []ProxyEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]ProxyEntry, len(p.proxies))
	copy(result, p.proxies)
	return result
}

// UpdateProxy updates an existing proxy's status and latency
func (p *ProxyPool) UpdateProxy(proxyURL string, status string, latencyMs int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.proxies {
		if p.proxies[i].URL == proxyURL {
			p.proxies[i].Status = status
			p.proxies[i].LatencyMs = latencyMs
			p.proxies[i].LastChecked = time.Now()
			return
		}
	}
}

// TestProxy tests a single proxy's connectivity and measures latency
func TestProxy(entry *ProxyEntry) error {
	start := time.Now()

	// Parse proxy URL
	proxyURL, err := url.Parse(entry.URL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	// Create HTTP client with proxy
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	// Test connectivity with a simple HTTP request
	testURL := "https://www.google.com"
	resp, err := client.Get(testURL)
	if err != nil {
		return fmt.Errorf("proxy test failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxy test returned status %d", resp.StatusCode)
	}

	// Calculate latency
	latency := time.Since(start)
	entry.LatencyMs = int(latency.Milliseconds())
	entry.Status = "ok"
	entry.LastChecked = time.Now()

	return nil
}

// Count returns the number of proxies in the pool
func (p *ProxyPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.proxies)
}
