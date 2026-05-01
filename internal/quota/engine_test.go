package quota

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/aegis-proxy/aegis/internal/database"
)

func TestQuotaEngineRecordsUsageAfterRequest(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.TrackUsage(ctx, "kiro", "account-1", 125); err != nil {
		t.Fatalf("TrackUsage() error = %v", err)
	}
	if err := store.TrackUsage(ctx, "kiro", "account-1", 75); err != nil {
		t.Fatalf("TrackUsage() second call error = %v", err)
	}

	quota, err := store.GetQuota(ctx, "kiro", "account-1")
	if err != nil {
		t.Fatalf("GetQuota() error = %v", err)
	}
	if quota.UsedTokens != 200 {
		t.Fatalf("UsedTokens = %d, want 200", quota.UsedTokens)
	}
}

func TestUnknownProviderQuotaStateMarkedStale(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.TrackUsage(ctx, "unknown", "account-1", 10); err != nil {
		t.Fatalf("TrackUsage() error = %v", err)
	}

	quota, err := store.GetQuota(ctx, "unknown", "account-1")
	if err != nil {
		t.Fatalf("GetQuota() error = %v", err)
	}
	if quota.Status != "unknown" || !quota.Stale {
		t.Fatalf("status/stale = %q/%v, want unknown/true", quota.Status, quota.Stale)
	}
}

func TestResetCountdownCalculation(t *testing.T) {
	store := newTestStore(t)
	resetAt := time.Now().UTC().Add(90 * time.Second)
	_, err := store.db.Exec(`
		INSERT INTO quotas (provider, account_id, used_tokens, total_tokens, reset_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "kiro", "account-1", 20, 100, resetAt, time.Now().UTC())
	if err != nil {
		t.Fatalf("insert quota: %v", err)
	}

	quota, err := store.GetQuota(context.Background(), "kiro", "account-1")
	if err != nil {
		t.Fatalf("GetQuota() error = %v", err)
	}
	if quota.ResetInSec <= 0 || quota.ResetInSec > 90 {
		t.Fatalf("ResetInSec = %d, want 1..90", quota.ResetInSec)
	}
	if quota.Remaining != 80 {
		t.Fatalf("Remaining = %d, want 80", quota.Remaining)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "quota.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewStore(db)
}
