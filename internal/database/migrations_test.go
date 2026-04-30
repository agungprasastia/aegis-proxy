package database

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestAutoMigrateFreshDBCreatesAllTables(t *testing.T) {
	db := openTestDB(t)

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	for _, table := range []string{
		"schema_migrations",
		"accounts",
		"request_logs",
		"settings",
		"proxies",
		"combos",
		"api_keys",
		"oauth_tokens",
		"quotas",
		"sync_metadata",
	} {
		if !tableExists(t, db, table) {
			t.Fatalf("expected table %q to exist", table)
		}
	}

	var versions int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&versions); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if versions != 9 {
		t.Fatalf("schema_migrations count = %d, want 9", versions)
	}
}

func TestAutoMigrateUpgradePreservesLegacyData(t *testing.T) {
	db := openTestDB(t)
	createLegacySchema(t, db)

	now := time.Now().UTC()
	result, err := db.Exec(`INSERT INTO accounts (
		email, password, provider, status, credits_used, credits_total, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, "user@example.com", "secret", "kiro", "active", 1.5, 10.0, now, now)
	if err != nil {
		t.Fatalf("insert legacy account: %v", err)
	}
	accountID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("legacy account id: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO request_logs (
		model, provider, account_id, status, status_code, prompt_tokens, completion_tokens,
		total_tokens, latency_ms, error_message, ip_address, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "claude-sonnet-4", "kiro", accountID, "ok", 200, 10, 20, 30, 40, "", "127.0.0.1", now); err != nil {
		t.Fatalf("insert legacy request_log: %v", err)
	}
	if _, err := db.Exec("INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)", "host", "127.0.0.1", now); err != nil {
		t.Fatalf("insert legacy setting: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	assertCount(t, db, "accounts", 1)
	assertCount(t, db, "request_logs", 1)
	assertCount(t, db, "settings", 1)

	var email, setting string
	if err := db.QueryRow("SELECT email FROM accounts WHERE id = ?", accountID).Scan(&email); err != nil {
		t.Fatalf("read preserved account: %v", err)
	}
	if email != "user@example.com" {
		t.Fatalf("preserved email = %q", email)
	}
	if err := db.QueryRow("SELECT value FROM settings WHERE key = ?", "host").Scan(&setting); err != nil {
		t.Fatalf("read preserved setting: %v", err)
	}
	if setting != "127.0.0.1" {
		t.Fatalf("preserved setting = %q", setting)
	}
}

func TestAutoMigrateSchemaQueryable(t *testing.T) {
	db := openTestDB(t)

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now().UTC()
	inserts := []string{
		`INSERT INTO combos (name, targets, created_at, updated_at) VALUES ('standard', '["kiro"]', ?, ?)`,
		`INSERT INTO api_keys (provider, key, status, created_at) VALUES ('kiro', 'encrypted-key', 'active', ?)`,
		`INSERT INTO oauth_tokens (provider, access_token, refresh_token, expires_at, created_at) VALUES ('kiro', 'encrypted-access', 'encrypted-refresh', ?, ?)`,
		`INSERT INTO quotas (provider, account_id, used_tokens, total_tokens, reset_at, created_at) VALUES ('kiro', NULL, 1, 100, ?, ?)`,
		`INSERT INTO sync_metadata (last_sync_at, last_sync_hash, created_at) VALUES (?, 'hash', ?)`,
	}
	args := [][]any{
		{now, now},
		{now},
		{now, now},
		{now, now},
		{now, now},
	}

	for i, insert := range inserts {
		if _, err := db.Exec(insert, args[i]...); err != nil {
			t.Fatalf("insert %d failed: %v", i, err)
		}
	}

	for _, table := range []string{"combos", "api_keys", "oauth_tokens", "quotas", "sync_metadata"} {
		assertCount(t, db, table, 1)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	return db
}

func createLegacySchema(t *testing.T, db *sql.DB) {
	t.Helper()
	legacy := []string{
		`CREATE TABLE accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL,
			password TEXT NOT NULL,
			provider TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			credits_used REAL NOT NULL DEFAULT 0,
			credits_total REAL NOT NULL DEFAULT 0,
			token TEXT,
			cookie TEXT,
			last_used_at DATETIME,
			last_synced_at DATETIME,
			error_message TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			model TEXT NOT NULL,
			provider TEXT NOT NULL,
			account_id INTEGER,
			status TEXT NOT NULL,
			status_code INTEGER,
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			ip_address TEXT,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
	}
	for _, statement := range legacy {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create legacy schema: %v", err)
		}
	}
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("check table %q: %v", table, err)
	}
	return name == table
}

func assertCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("count %s = %d, want %d", table, got, want)
	}
}
