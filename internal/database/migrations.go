package database

import (
	"database/sql"
	"fmt"
)

func AutoMigrate(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
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
		`CREATE TABLE IF NOT EXISTS request_logs (
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
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS proxies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			last_tested_at DATETIME,
			created_at DATETIME NOT NULL
		)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}
