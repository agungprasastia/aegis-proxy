package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aegis-proxy/aegis/internal/accounts"
	"github.com/aegis-proxy/aegis/internal/auth"
	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
)

// BatchStatus represents the current state of a batch job
type BatchStatus struct {
	Active  bool       `json:"active"`
	Status  string     `json:"status"` // "idle", "running", "completed", "cancelled"
	Current int        `json:"current"`
	Total   int        `json:"total"`
	Success int        `json:"success"`
	Failed  int        `json:"failed"`
	Logs    []LogEntry `json:"logs"`
}

// LogEntry represents a single log message
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Level     string    `json:"level"` // "info", "warn", "error", "success"
}

// AccountInput represents an account to be added
type AccountInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// BatchConfig holds configuration for the batch job
type BatchConfig struct {
	Concurrent int    `json:"concurrent"`
	Headless   bool   `json:"headless"`
	Priority   string `json:"priority"` // which provider to prioritize
}

// BatchManager manages batch account login jobs
type BatchManager struct {
	mu      sync.RWMutex
	status  BatchStatus
	cancel  context.CancelFunc
	am      *accounts.AccountManager
}

// NewBatchManager creates a new BatchManager
func NewBatchManager(am *accounts.AccountManager) *BatchManager {
	return &BatchManager{
		am: am,
		status: BatchStatus{
			Status: "idle",
			Logs:   []LogEntry{},
		},
	}
}

// log adds a log entry (thread-safe)
func (bm *BatchManager) log(level, message string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.status.Logs = append(bm.status.Logs, LogEntry{
		Timestamp: time.Now(),
		Message:   message,
		Level:     level,
	})
}

// GetStatus returns the current batch status
func (bm *BatchManager) GetStatus() BatchStatus {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	// Return a copy
	logsCopy := make([]LogEntry, len(bm.status.Logs))
	copy(logsCopy, bm.status.Logs)

	return BatchStatus{
		Active:  bm.status.Active,
		Status:  bm.status.Status,
		Current: bm.status.Current,
		Total:   bm.status.Total,
		Success: bm.status.Success,
		Failed:  bm.status.Failed,
		Logs:    logsCopy,
	}
}

// GetLogs returns all log entries
func (bm *BatchManager) GetLogs() []LogEntry {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	logsCopy := make([]LogEntry, len(bm.status.Logs))
	copy(logsCopy, bm.status.Logs)
	return logsCopy
}

// Cancel cancels the running batch job
func (bm *BatchManager) Cancel() {
	bm.mu.Lock()
	if bm.cancel != nil {
		bm.cancel()
	}
	bm.status.Status = "cancelled"
	bm.status.Active = false
	bm.mu.Unlock()

	bm.log("warn", "Batch cancelled by user")
}

// Start begins processing a batch of accounts
func (bm *BatchManager) Start(inputAccounts []AccountInput, cfg BatchConfig) error {
	bm.mu.Lock()
	if bm.status.Active {
		bm.mu.Unlock()
		return fmt.Errorf("a batch job is already running")
	}

	// Reset state
	bm.status = BatchStatus{
		Active:  true,
		Status:  "running",
		Current: 0,
		Total:   len(inputAccounts),
		Success: 0,
		Failed:  0,
		Logs:    []LogEntry{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	bm.cancel = cancel
	bm.mu.Unlock()

	bm.log("info", fmt.Sprintf("Starting batch add of %d accounts...", len(inputAccounts)))

	// Run in background goroutine
	go bm.processBatch(ctx, inputAccounts, cfg)

	return nil
}

// processBatch processes all accounts in the batch
func (bm *BatchManager) processBatch(ctx context.Context, inputAccounts []AccountInput, cfg BatchConfig) {
	defer func() {
		bm.mu.Lock()
		bm.status.Active = false
		if bm.status.Status == "running" {
			bm.status.Status = "completed"
		}
		bm.mu.Unlock()

		bm.log("info", fmt.Sprintf("Batch complete: %d success, %d failed out of %d",
			bm.status.Success, bm.status.Failed, bm.status.Total))
	}()

	for i, acc := range inputAccounts {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		bm.mu.Lock()
		bm.status.Current = i + 1
		bm.mu.Unlock()

		bm.log("info", fmt.Sprintf("[%d/%d] Adding %s...", i+1, len(inputAccounts), acc.Email))

		success := bm.processOneAccount(ctx, acc, cfg)

		bm.mu.Lock()
		if success {
			bm.status.Success++
		} else {
			bm.status.Failed++
		}
		bm.mu.Unlock()

		if success {
			bm.log("success", fmt.Sprintf("[%d/%d] %s — added successfully", i+1, len(inputAccounts), acc.Email))
		} else {
			bm.log("error", fmt.Sprintf("[%d/%d] %s — failed", i+1, len(inputAccounts), acc.Email))
		}
	}
}

// processOneAccount handles login for a single account
func (bm *BatchManager) processOneAccount(ctx context.Context, acc AccountInput, cfg BatchConfig) bool {
	bm.log("info", fmt.Sprintf("→ Starting login for %s...", acc.Email))

	// Check for cancellation
	select {
	case <-ctx.Done():
		return false
	default:
	}

	// Run Python login with options
	opts := auth.LoginOptions{
		Headless:   cfg.Headless,
		Concurrent: cfg.Concurrent,
		Priority:   cfg.Priority,
	}

	result, err := auth.RunLoginWithOptions(acc.Email, acc.Password, opts, func(event auth.ProgressEvent) {
		providerTag := ""
		if event.Provider != "" {
			providerTag = fmt.Sprintf("[%s] ", event.Provider)
		}
		bm.log("info", fmt.Sprintf("→ %s%s", providerTag, event.Message))
	})

	if err != nil {
		bm.log("error", fmt.Sprintf("→ Login failed for %s: %v", acc.Email, err))
		return false
	}

	if result == nil {
		bm.log("error", fmt.Sprintf("→ No result from login for %s", acc.Email))
		return false
	}

	// Process each provider's credentials
	anySuccess := false
	providers := []struct {
		name  string
		creds *auth.ProviderCredentials
	}{
		{"kiro", result.Kiro},
		{"codebuddy", result.CodeBuddy},
		{"wavespeed", result.Wavespeed},
		{"canva", result.Canva},
		{"yepapi", result.YepAPI},
	}

	for _, p := range providers {
		if p.creds == nil || !p.creds.Success {
			if p.creds != nil && p.creds.Error != "" {
				bm.log("warn", fmt.Sprintf("⚠ [%s] %s", p.name, p.creds.Error))
			} else {
				bm.log("warn", fmt.Sprintf("⚠ [%s] skipped", p.name))
			}
			continue
		}

		// Serialize full credentials as JSON for account.Token
		// Providers expect Token to be JSON: {"access_token":"...","refresh_token":"...",...}
		tokenJSON := ""
		cookie := ""
		if p.creds.Credentials != nil {
			if credBytes, err := json.Marshal(p.creds.Credentials); err == nil {
				tokenJSON = string(credBytes)
			}
			// Extract cookie if present
			for _, key := range []string{"cookie", "cookies", "session", "all_cookies"} {
				if c, ok := p.creds.Credentials[key].(string); ok && c != "" {
					cookie = c
					break
				}
			}
		}

		// Add account to database
		dbAcc, err := bm.am.Add(acc.Email, acc.Password, p.name)
		if err != nil {
			bm.log("error", fmt.Sprintf("→ [%s] Failed to add to database: %v", p.name, err))
			continue
		}

		// Update token in DB and in-memory object
		if err := bm.am.UpdateToken(dbAcc.ID, tokenJSON, cookie); err != nil {
			bm.log("warn", fmt.Sprintf("→ [%s] Failed to update token: %v", p.name, err))
		}
		dbAcc.Token = tokenJSON
		dbAcc.Cookie = cookie

		// Update credits from quota
		if p.creds.Quota != nil {
			var creditsTotal, creditsUsed float64
			if total, ok := p.creds.Quota["total_credits"].(float64); ok {
				creditsTotal = total
			} else if total, ok := p.creds.Quota["limit"].(float64); ok {
				creditsTotal = total
			}
			if remaining, ok := p.creds.Quota["remaining_credits"].(float64); ok {
				creditsUsed = creditsTotal - remaining
			} else if remaining, ok := p.creds.Quota["remaining"].(float64); ok {
				creditsUsed = creditsTotal - remaining
			}
			if creditsTotal > 0 {
				dbAcc.CreditsTotal = creditsTotal
				dbAcc.CreditsUsed = creditsUsed
				_ = bm.am.UpdateCredits(dbAcc.ID, creditsUsed, creditsTotal)
				bm.log("info", fmt.Sprintf("→ [%s] Quota: %.0f/%.0f credits remaining", p.name, creditsTotal-creditsUsed, creditsTotal))
			}
		}

		// Warmup
		prov, ok := provider.ProviderRegistry[p.name]
		if ok {
			bm.log("info", fmt.Sprintf("→ [%s] Warming up...", p.name))
			if err := accounts.WarmupAccount(prov, dbAcc, bm.am.DB()); err != nil {
				bm.log("warn", fmt.Sprintf("→ [%s] Warmup failed: %v", p.name, err))
				_ = bm.am.UpdateStatus(dbAcc.ID, models.StatusError, fmt.Sprintf("Warmup failed: %v", err))
			} else {
				_ = bm.am.UpdateStatus(dbAcc.ID, models.StatusActive, "")
				bm.log("success", fmt.Sprintf("→ [%s] Active", p.name))
			}
		} else {
			_ = bm.am.UpdateStatus(dbAcc.ID, models.StatusActive, "")
		}

		anySuccess = true
	}

	return anySuccess
}
