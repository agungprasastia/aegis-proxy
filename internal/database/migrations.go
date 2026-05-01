package database

import (
	"database/sql"
	"fmt"
	"time"
)

type migration struct {
	version int
	query   string
}

func AutoMigrate(db *sql.DB) error {
	migrations := []migration{
		{1, `CREATE TABLE IF NOT EXISTS accounts (
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
		)`},
		{2, `CREATE TABLE IF NOT EXISTS request_logs (
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
		)`},
		{3, `CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		)`},
		{4, `CREATE TABLE IF NOT EXISTS proxies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			last_tested_at DATETIME,
			created_at DATETIME NOT NULL
		)`},
		{5, `CREATE TABLE IF NOT EXISTS combos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			targets TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`},
		{6, `CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			key TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL
		)`},
		{7, `CREATE TABLE IF NOT EXISTS oauth_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			expires_at DATETIME,
			created_at DATETIME NOT NULL
		)`},
		{8, `CREATE TABLE IF NOT EXISTS quotas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			account_id TEXT NOT NULL,
			used_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			reset_at DATETIME,
			created_at DATETIME NOT NULL,
			UNIQUE(provider, account_id)
		)`},
		{9, `CREATE TABLE IF NOT EXISTS sync_metadata (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			last_sync_at DATETIME,
			last_sync_hash TEXT,
			created_at DATETIME NOT NULL
		)`},
		{10, `CREATE UNIQUE INDEX IF NOT EXISTS idx_quotas_provider_account ON quotas(provider, account_id)`},
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL
	)`); err != nil {
		return fmt.Errorf("failed to create schema_migrations: %w", err)
	}

	for _, migration := range migrations {
		var exists int
		if err := db.QueryRow("SELECT 1 FROM schema_migrations WHERE version = ?", migration.version).Scan(&exists); err == nil {
			continue
		} else if err != sql.ErrNoRows {
			return fmt.Errorf("failed to check migration version %d: %w", migration.version, err)
		}

		if _, err := db.Exec(migration.query); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}

		if _, err := db.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", migration.version, time.Now()); err != nil {
			return fmt.Errorf("failed to record migration version %d: %w", migration.version, err)
		}
	}

	return nil
}
