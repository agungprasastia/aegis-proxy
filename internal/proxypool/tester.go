package proxypool

import (
	"log"
	"sync"
	"time"
)

// ProxyTester manages background testing of proxies
type ProxyTester struct {
	pool             *ProxyPool
	config           *ProxyPoolConfig
	dataDir          string
	stopChan         chan struct{}
	wg               sync.WaitGroup
	running          bool
	mu               sync.Mutex
}

// NewProxyTester creates a new proxy tester
func NewProxyTester(pool *ProxyPool, config *ProxyPoolConfig, dataDir string) *ProxyTester {
	return &ProxyTester{
		pool:     pool,
		config:   config,
		dataDir:  dataDir,
		stopChan: make(chan struct{}),
		running:  false,
	}
}

// Start begins the background proxy testing goroutine
func (t *ProxyTester) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return
	}

	if !t.config.AutoTestEnabled {
		log.Println("Proxy auto-testing is disabled")
		return
	}

	t.running = true
	t.wg.Add(1)

	go t.testLoop()
	log.Printf("Proxy tester started with %d minute interval", t.config.AutoTestIntervalMin)
}

// Stop stops the background proxy testing goroutine
func (t *ProxyTester) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return
	}

	close(t.stopChan)
	t.wg.Wait()
	t.running = false
	log.Println("Proxy tester stopped")
}

// testLoop is the main testing loop that runs in the background
func (t *ProxyTester) testLoop() {
	defer t.wg.Done()

	// Calculate interval
	interval := time.Duration(t.config.AutoTestIntervalMin) * time.Minute
	if interval < 1*time.Minute {
		interval = 5 * time.Minute // Default to 5 minutes
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run initial test immediately
	t.testAllProxies()

	for {
		select {
		case <-t.stopChan:
			return
		case <-ticker.C:
			t.testAllProxies()
		}
	}
}

// testAllProxies tests all proxies in the pool
func (t *ProxyTester) testAllProxies() {
	proxies := t.pool.GetAllProxies()
	
	if len(proxies) == 0 {
		return
	}

	log.Printf("Testing %d proxies...", len(proxies))
	
	var wg sync.WaitGroup
	failedProxies := make([]string, 0)
	var failedMu sync.Mutex

	for i := range proxies {
		wg.Add(1)
		go func(proxy ProxyEntry) {
			defer wg.Done()
			
			// Test the proxy
			err := TestProxy(&proxy)
			
			if err != nil {
				log.Printf("Proxy %s failed: %v", proxy.URL, err)
				t.pool.UpdateProxy(proxy.URL, "failed", 0)
				
				// Track failed proxy for potential deletion
				if t.config.AutoDeleteFailed {
					failedMu.Lock()
					failedProxies = append(failedProxies, proxy.URL)
					failedMu.Unlock()
				}
			} else {
				log.Printf("Proxy %s OK (latency: %dms)", proxy.URL, proxy.LatencyMs)
				t.pool.UpdateProxy(proxy.URL, proxy.Status, proxy.LatencyMs)
			}
		}(proxies[i])
	}

	// Wait for all tests to complete
	wg.Wait()

	// Delete failed proxies if configured
	if t.config.AutoDeleteFailed && len(failedProxies) > 0 {
		for _, proxyURL := range failedProxies {
			log.Printf("Auto-deleting failed proxy: %s", proxyURL)
			t.pool.RemoveProxy(proxyURL)
		}
	}

	// Save updated pool to disk
	if err := SaveProxyPool(t.dataDir, t.pool, t.config); err != nil {
		log.Printf("Failed to save proxy pool: %v", err)
	}

	log.Printf("Proxy testing complete. Active proxies: %d", t.pool.Count())
}

// TestNow triggers an immediate test of all proxies
func (t *ProxyTester) TestNow() {
	go t.testAllProxies()
}

// IsRunning returns whether the tester is currently running
func (t *ProxyTester) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// UpdateConfig updates the tester's configuration
// If auto-testing was disabled and is now enabled, starts the tester
// If interval changed, restarts the tester
func (t *ProxyTester) UpdateConfig(config *ProxyPoolConfig) {
	t.mu.Lock()
	oldEnabled := t.config.AutoTestEnabled
	oldInterval := t.config.AutoTestIntervalMin
	t.config = config
	t.mu.Unlock()

	// Restart if needed
	if config.AutoTestEnabled && (!oldEnabled || oldInterval != config.AutoTestIntervalMin) {
		if t.IsRunning() {
			t.Stop()
		}
		t.Start()
	} else if !config.AutoTestEnabled && oldEnabled {
		t.Stop()
	}
}
