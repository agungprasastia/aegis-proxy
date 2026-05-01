package quota

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aegis-proxy/aegis/internal/database"
)

type QuotaTracker struct {
	Provider    string     `json:"provider"`
	AccountID   string     `json:"account_id"`
	UsedTokens  int        `json:"used_tokens"`
	TotalTokens int        `json:"total_tokens"`
	ResetAt     *time.Time `json:"reset_at,omitempty"`
	Status      string     `json:"status"`
	Stale       bool       `json:"stale"`
	Remaining   int        `json:"remaining_tokens"`
	ResetInSec  int64      `json:"reset_in_seconds"`
}

type Store struct {
	db *database.DB
}

var (
	defaultMu    sync.RWMutex
	defaultStore *Store
)

func NewStore(db *database.DB) *Store {
	return &Store{db: db}
}

func SetStore(db *database.DB) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultStore = NewStore(db)
}

func TrackUsage(ctx context.Context, provider, accountID string, tokens int) error {
	store, err := getDefaultStore()
	if err != nil {
		return err
	}
	return store.TrackUsage(ctx, provider, accountID, tokens)
}

func GetQuota(ctx context.Context, provider, accountID string) (*QuotaTracker, error) {
	store, err := getDefaultStore()
	if err != nil {
		return nil, err
	}
	return store.GetQuota(ctx, provider, accountID)
}

func ListQuotas(ctx context.Context, provider string) ([]*QuotaTracker, error) {
	store, err := getDefaultStore()
	if err != nil {
		return nil, err
	}
	return store.ListQuotas(ctx, provider)
}

func (s *Store) TrackUsage(ctx context.Context, provider, accountID string, tokens int) error {
	if s == nil || s.db == nil {
		return errors.New("quota store is not configured")
	}
	if provider == "" || accountID == "" {
		return errors.New("provider and account ID are required")
	}
	if tokens < 0 {
		return errors.New("tokens cannot be negative")
	}

	now := time.Now().UTC()
	_, err := s.db.SQL().ExecContext(ctx, `
		INSERT INTO quotas (provider, account_id, used_tokens, total_tokens, reset_at, created_at)
		VALUES (?, ?, ?, 0, NULL, ?)
		ON CONFLICT(provider, account_id) DO UPDATE SET used_tokens = used_tokens + excluded.used_tokens
	`, provider, accountID, tokens, now)
	if err != nil {
		return fmt.Errorf("failed to track quota usage: %w", err)
	}
	return nil
}

func (s *Store) GetQuota(ctx context.Context, provider, accountID string) (*QuotaTracker, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("quota store is not configured")
	}
	if provider == "" || accountID == "" {
		return nil, errors.New("provider and account ID are required")
	}

	quota, err := scanQuota(s.db.SQL().QueryRowContext(ctx, `
		SELECT provider, account_id, used_tokens, total_tokens, reset_at
		FROM quotas WHERE provider = ? AND account_id = ?
	`, provider, accountID))
	if err != nil {
		return nil, err
	}
	return quota, nil
}

func (s *Store) ListQuotas(ctx context.Context, provider string) ([]*QuotaTracker, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("quota store is not configured")
	}
	if provider == "" {
		return nil, errors.New("provider is required")
	}

	rows, err := s.db.SQL().QueryContext(ctx, `
		SELECT provider, account_id, used_tokens, total_tokens, reset_at
		FROM quotas WHERE provider = ? ORDER BY account_id
	`, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to list quotas: %w", err)
	}
	defer rows.Close()

	quotas := []*QuotaTracker{}
	for rows.Next() {
		quota, err := scanQuota(rows)
		if err != nil {
			return nil, err
		}
		quotas = append(quotas, quota)
	}
	return quotas, rows.Err()
}

func (q *QuotaTracker) Exhausted() bool {
	return q != nil && !q.Stale && q.TotalTokens > 0 && q.UsedTokens >= q.TotalTokens
}

func getDefaultStore() (*Store, error) {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	if defaultStore == nil || defaultStore.db == nil {
		return nil, errors.New("quota store is not configured")
	}
	return defaultStore, nil
}

type quotaScanner interface {
	Scan(dest ...interface{}) error
}

func scanQuota(scanner quotaScanner) (*QuotaTracker, error) {
	var q QuotaTracker
	var reset sql.NullTime
	if err := scanner.Scan(&q.Provider, &q.AccountID, &q.UsedTokens, &q.TotalTokens, &reset); err != nil {
		return nil, err
	}
	if reset.Valid {
		q.ResetAt = &reset.Time
		q.ResetInSec = int64(time.Until(reset.Time).Seconds())
		if q.ResetInSec < 0 {
			q.ResetInSec = 0
		}
	}
	if q.TotalTokens > 0 {
		q.Remaining = q.TotalTokens - q.UsedTokens
		if q.Remaining < 0 {
			q.Remaining = 0
		}
		if q.Exhausted() {
			q.Status = "exhausted"
		} else {
			q.Status = "known"
		}
		return &q, nil
	}

	q.Status = "unknown"
	q.Stale = true
	return &q, nil
}
