package accounts

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
)

type AccountManager struct {
	db            *database.DB
	mu            sync.Mutex
	roundRobinIdx map[string]int
	stickyMap     map[string]int64 // key: "provider+clientID", value: accountID
	stickyPath    string
	lastErrorTime map[int64]time.Time // track when account last errored
}

func NewAccountManager(db *database.DB) *AccountManager {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	dataDir := filepath.Join(homeDir, ".aegis-proxy")
	stickyPath := filepath.Join(dataDir, "sticky-sessions.json")

	am := &AccountManager{
		db:            db,
		roundRobinIdx: make(map[string]int),
		stickyMap:     make(map[string]int64),
		stickyPath:    stickyPath,
		lastErrorTime: make(map[int64]time.Time),
	}

	if err := am.LoadStickyMap(); err != nil {
		log.Printf("Warning: failed to load sticky map: %v", err)
	}

	// Start background recovery goroutine
	go am.autoRecoveryLoop()

	return am
}

func (am *AccountManager) GetAll(provider string) ([]*models.Account, error) {
	query := `
		SELECT id, email, password, provider, status, credits_used, credits_total,
		       token, cookie, last_used_at, last_synced_at, error_message, created_at, updated_at
		FROM accounts
	`
	args := []interface{}{}

	if provider != "" {
		query += " WHERE provider = ?"
		args = append(args, provider)
	}

	query += " ORDER BY created_at DESC"

	rows, err := am.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*models.Account
	for rows.Next() {
		var acc models.Account
		if err := rows.Scan(
			&acc.ID, &acc.Email, &acc.Password, &acc.Provider, &acc.Status,
			&acc.CreditsUsed, &acc.CreditsTotal, &acc.Token, &acc.Cookie,
			&acc.LastUsedAt, &acc.LastSyncedAt, &acc.ErrorMessage,
			&acc.CreatedAt, &acc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}
		accounts = append(accounts, &acc)
	}

	return accounts, rows.Err()
}

func (am *AccountManager) GetByID(id int64) (*models.Account, error) {
	var acc models.Account
	err := am.db.QueryRow(`
		SELECT id, email, password, provider, status, credits_used, credits_total,
		       token, cookie, last_used_at, last_synced_at, error_message, created_at, updated_at
		FROM accounts WHERE id = ?
	`, id).Scan(
		&acc.ID, &acc.Email, &acc.Password, &acc.Provider, &acc.Status,
		&acc.CreditsUsed, &acc.CreditsTotal, &acc.Token, &acc.Cookie,
		&acc.LastUsedAt, &acc.LastSyncedAt, &acc.ErrorMessage,
		&acc.CreatedAt, &acc.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account not found")
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	return &acc, nil
}

func (am *AccountManager) GetAvailable(provider string) (*models.Account, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	accounts, err := am.getActiveAccounts(provider)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no active accounts available for provider: %s", provider)
	}

	key := provider
	if key == "" {
		key = "all"
	}

	idx := am.roundRobinIdx[key] % len(accounts)
	am.roundRobinIdx[key] = (idx + 1) % len(accounts)

	return accounts[idx], nil
}

// GetSticky returns a sticky account for a specific client.
// If the client already has a sticky account, it returns that account.
// If the sticky account is error/exhausted, it rotates to a new account.
// If the client doesn't have a sticky account, it assigns a new one.
func (am *AccountManager) GetSticky(provider, clientID string) (*models.Account, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	key := provider + "+" + clientID

	// Check if client already has a sticky account
	if accountID, exists := am.stickyMap[key]; exists {
		// Try to get the sticky account
		account, err := am.GetByID(accountID)
		if err == nil && account.Status == models.StatusActive {
			// Sticky account is still active, return it
			return account, nil
		}

		// Sticky account is error/exhausted, need to rotate
		accounts, err := am.getActiveAccounts(provider)
		if err != nil {
			return nil, err
		}

		if len(accounts) == 0 {
			return nil, fmt.Errorf("no active accounts available for provider: %s", provider)
		}

		// Find a different account (not the failed one)
		var newAccount *models.Account
		for _, acc := range accounts {
			if acc.ID != accountID {
				newAccount = acc
				break
			}
		}

		// If all accounts are the same (only one account), use it anyway
		if newAccount == nil {
			newAccount = accounts[0]
		}

		// Update sticky map
		am.stickyMap[key] = newAccount.ID

		// Save sticky map to disk
		if err := am.SaveStickyMap(); err != nil {
			log.Printf("Warning: failed to save sticky map: %v", err)
		}

		// Log rotation
		log.Printf("sticky account rotated provider=%s email=%s", provider, newAccount.Email)

		return newAccount, nil
	}

	// No sticky account yet, assign a new one
	accounts, err := am.getActiveAccounts(provider)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no active accounts available for provider: %s", provider)
	}

	// Use round-robin to select initial account
	rrKey := provider
	if rrKey == "" {
		rrKey = "all"
	}

	idx := am.roundRobinIdx[rrKey] % len(accounts)
	am.roundRobinIdx[rrKey] = (idx + 1) % len(accounts)

	selectedAccount := accounts[idx]
	am.stickyMap[key] = selectedAccount.ID

	// Save sticky map to disk
	if err := am.SaveStickyMap(); err != nil {
		log.Printf("Warning: failed to save sticky map: %v", err)
	}

	return selectedAccount, nil
}

func (am *AccountManager) getActiveAccounts(provider string) ([]*models.Account, error) {
	query := `
		SELECT id, email, password, provider, status, credits_used, credits_total,
		       token, cookie, last_used_at, last_synced_at, error_message, created_at, updated_at
		FROM accounts
		WHERE status = ?
	`
	args := []interface{}{models.StatusActive}

	if provider != "" {
		query += " AND provider = ?"
		args = append(args, provider)
	}

	query += " ORDER BY last_used_at ASC NULLS FIRST"

	rows, err := am.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query active accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*models.Account
	for rows.Next() {
		var acc models.Account
		if err := rows.Scan(
			&acc.ID, &acc.Email, &acc.Password, &acc.Provider, &acc.Status,
			&acc.CreditsUsed, &acc.CreditsTotal, &acc.Token, &acc.Cookie,
			&acc.LastUsedAt, &acc.LastSyncedAt, &acc.ErrorMessage,
			&acc.CreatedAt, &acc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}
		accounts = append(accounts, &acc)
	}

	return accounts, rows.Err()
}

func (am *AccountManager) Add(email, password, provider string) (*models.Account, error) {
	now := time.Now()
	acc := &models.Account{
		Email:     email,
		Password:  password,
		Provider:  provider,
		Status:    models.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	result, err := am.db.Exec(`
		INSERT INTO accounts (email, password, provider, status, credits_used, credits_total,
		                      token, cookie, last_used_at, last_synced_at, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, acc.Email, acc.Password, acc.Provider, acc.Status, acc.CreditsUsed, acc.CreditsTotal,
		acc.Token, acc.Cookie, acc.LastUsedAt, acc.LastSyncedAt, acc.ErrorMessage, acc.CreatedAt, acc.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert account: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}
	acc.ID = id

	return acc, nil
}

func (am *AccountManager) BatchAdd(provider, text string) (int, error) {
	lines := strings.Split(text, "\n")
	count := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		email := strings.TrimSpace(parts[0])
		password := strings.TrimSpace(parts[1])

		if email == "" || password == "" {
			continue
		}

		_, err := am.Add(email, password, provider)
		if err != nil {
			return count, fmt.Errorf("failed to add account %s: %w", email, err)
		}
		count++
	}

	return count, nil
}

func (am *AccountManager) Remove(id int64) error {
	result, err := am.db.Exec("DELETE FROM accounts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

func (am *AccountManager) UpdateStatus(id int64, status, errorMsg string) error {
	now := time.Now()
	_, err := am.db.Exec(`
		UPDATE accounts SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`, status, errorMsg, now, id)
	if err != nil {
		return fmt.Errorf("failed to update account status: %w", err)
	}
	return nil
}

func (am *AccountManager) UpdateCredits(id int64, used, total float64) error {
	now := time.Now()
	_, err := am.db.Exec(`
		UPDATE accounts SET credits_used = ?, credits_total = ?, updated_at = ?
		WHERE id = ?
	`, used, total, now, id)
	if err != nil {
		return fmt.Errorf("failed to update account credits: %w", err)
	}
	return nil
}

func (am *AccountManager) UpdateToken(id int64, token, cookie string) error {
	now := time.Now()
	_, err := am.db.Exec(`
		UPDATE accounts SET token = ?, cookie = ?, updated_at = ?
		WHERE id = ?
	`, token, cookie, now, id)
	if err != nil {
		return fmt.Errorf("failed to update account token: %w", err)
	}
	return nil
}

func (am *AccountManager) MarkUsed(id int64) error {
	now := time.Now()
	_, err := am.db.Exec(`
		UPDATE accounts SET last_used_at = ?, updated_at = ?
		WHERE id = ?
	`, now, now, id)
	if err != nil {
		return fmt.Errorf("failed to mark account as used: %w", err)
	}
	return nil
}

func (am *AccountManager) GetStats(provider string) (active, exhausted, banned, errorCount, total int, err error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM accounts
	`
	args := []interface{}{}

	if provider != "" {
		query += " WHERE provider = ?"
		args = append(args, provider)
	}

	query += " GROUP BY status"

	rows, err := am.db.Query(query, args...)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}

		total += count
		switch status {
		case models.StatusActive:
			active = count
		case models.StatusExhausted:
			exhausted = count
		case models.StatusBanned:
			banned = count
		case models.StatusError:
			errorCount = count
		}
	}

	return active, exhausted, banned, errorCount, total, rows.Err()
}

func (am *AccountManager) DeleteInactive() (int, error) {
	result, err := am.db.Exec("DELETE FROM accounts WHERE status != ?", models.StatusActive)
	if err != nil {
		return 0, fmt.Errorf("failed to delete inactive accounts: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rows), nil
}

func (am *AccountManager) DeleteAll() (int, error) {
	result, err := am.db.Exec("DELETE FROM accounts")
	if err != nil {
		return 0, fmt.Errorf("failed to delete all accounts: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rows), nil
}

// GetSticky returns a sticky account for the given provider and clientID.
// If the client already has a sticky account, it returns the same account.
// If the sticky account is unavailable or errored, it rotates to a new account.
func (am *AccountManager) rotateStickyAccount(provider, clientID string) (*models.Account, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	key := fmt.Sprintf("%s+%s", provider, clientID)
	return am.rotateStickyAccountLocked(provider, clientID, key)
}

// rotateStickyAccountLocked is the internal implementation that assumes the lock is already held.
func (am *AccountManager) rotateStickyAccountLocked(provider, clientID, key string) (*models.Account, error) {
	// Remove old entry
	delete(am.stickyMap, key)

	// Get new account
	accounts, err := am.getActiveAccounts(provider)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no active accounts available for provider: %s", provider)
	}

	// Use round-robin to select new account
	rrKey := provider
	if rrKey == "" {
		rrKey = "all"
	}
	idx := am.roundRobinIdx[rrKey] % len(accounts)
	am.roundRobinIdx[rrKey] = (idx + 1) % len(accounts)

	newAccount := accounts[idx]
	am.stickyMap[key] = newAccount.ID

	if err := am.SaveStickyMap(); err != nil {
		log.Printf("Warning: failed to save sticky map: %v", err)
	}

	return newAccount, nil
}

// LoadStickyMap loads the sticky session map from disk.
func (am *AccountManager) LoadStickyMap() error {
	data, err := os.ReadFile(am.stickyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's OK
		}
		return fmt.Errorf("failed to read sticky map file: %w", err)
	}

	if err := json.Unmarshal(data, &am.stickyMap); err != nil {
		return fmt.Errorf("failed to unmarshal sticky map: %w", err)
	}

	return nil
}

// SaveStickyMap saves the sticky session map to disk.
func (am *AccountManager) SaveStickyMap() error {
	data, err := json.MarshalIndent(am.stickyMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sticky map: %w", err)
	}

	if err := os.WriteFile(am.stickyPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write sticky map file: %w", err)
	}

	return nil
}

// MarkError marks an account as error and records the error time.
func (am *AccountManager) MarkError(accountID int64, errorMsg string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Update account status to error
	if err := am.UpdateStatus(accountID, models.StatusError, errorMsg); err != nil {
		return err
	}

	// Record error timestamp
	am.lastErrorTime[accountID] = time.Now()

	return nil
}

// TryRecover attempts to recover an errored account.
// Returns true if recovery was successful.
func (am *AccountManager) TryRecover(accountID int64) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Get account
	account, err := am.GetByID(accountID)
	if err != nil {
		return false
	}

	// Only try to recover error status accounts
	if account.Status != models.StatusError {
		return false
	}

	// Check cooldown: don't retry if error happened < 2 minutes ago
	if lastError, exists := am.lastErrorTime[accountID]; exists {
		if time.Since(lastError) < 2*time.Minute {
			return false
		}
	}

	// Try to validate account with a simple test
	// For now, we just check if token/cookie exists
	// In a real implementation, you'd send a small test request to the provider
	if account.Token == "" && account.Cookie == "" {
		// No credentials to test, update error time and return
		am.lastErrorTime[accountID] = time.Now()
		return false
	}

	// Simulate successful validation (in real implementation, test with provider)
	// For now, we assume if credentials exist, recovery is possible
	if err := am.UpdateStatus(accountID, models.StatusActive, ""); err != nil {
		am.lastErrorTime[accountID] = time.Now()
		return false
	}

	// Log successful recovery
	log.Printf("account auto-recovered email=%s from=error", account.Email)

	// Remove from error tracking
	delete(am.lastErrorTime, accountID)

	return true
}

// autoRecoveryLoop runs in the background and periodically tries to recover errored accounts.
func (am *AccountManager) autoRecoveryLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		am.tryRecoverAllErrorAccounts()
	}
}

// tryRecoverAllErrorAccounts attempts to recover all accounts in error status.
func (am *AccountManager) tryRecoverAllErrorAccounts() {
	// Get all accounts with error status
	accounts, err := am.GetAll("")
	if err != nil {
		log.Printf("auto-recovery: failed to get accounts: %v", err)
		return
	}

	recoveredCount := 0
	attemptedCount := 0

	for _, account := range accounts {
		if account.Status == models.StatusError {
			attemptedCount++
			if am.TryRecover(account.ID) {
				recoveredCount++
			}
		}
	}

	if attemptedCount > 0 {
		log.Printf("auto-recovery: attempted=%d recovered=%d", attemptedCount, recoveredCount)
	}
}

// DB returns the underlying database connection.
func (am *AccountManager) DB() *database.DB {
	return am.db
}

// GetProvider returns a provider instance for the given provider name.
func (am *AccountManager) GetProvider(providerName string) (provider.Provider, error) {
	prov, exists := provider.ProviderRegistry[providerName]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}
	return prov, nil
}
