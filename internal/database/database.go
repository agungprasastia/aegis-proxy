package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/aegis-proxy/aegis/internal/models"
	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func InitDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := AutoMigrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.Query(query, args...)
}

func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRow(query, args...)
}

func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.conn.Exec(query, args...)
}

func (db *DB) GetAccount(id int64) (*models.Account, error) {
	var acc models.Account
	err := db.conn.QueryRow(`
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
		return nil, err
	}
	return &acc, nil
}

func (db *DB) SaveAccount(acc *models.Account) error {
	now := time.Now()
	if acc.ID == 0 {
		acc.CreatedAt = now
		acc.UpdatedAt = now
		result, err := db.conn.Exec(`
			INSERT INTO accounts (email, password, provider, status, credits_used, credits_total,
			                      token, cookie, last_used_at, last_synced_at, error_message, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, acc.Email, acc.Password, acc.Provider, acc.Status, acc.CreditsUsed, acc.CreditsTotal,
			acc.Token, acc.Cookie, acc.LastUsedAt, acc.LastSyncedAt, acc.ErrorMessage, acc.CreatedAt, acc.UpdatedAt)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		acc.ID = id
		return nil
	}

	acc.UpdatedAt = now
	_, err := db.conn.Exec(`
		UPDATE accounts SET email=?, password=?, provider=?, status=?, credits_used=?, credits_total=?,
		                    token=?, cookie=?, last_used_at=?, last_synced_at=?, error_message=?, updated_at=?
		WHERE id=?
	`, acc.Email, acc.Password, acc.Provider, acc.Status, acc.CreditsUsed, acc.CreditsTotal,
		acc.Token, acc.Cookie, acc.LastUsedAt, acc.LastSyncedAt, acc.ErrorMessage, acc.UpdatedAt, acc.ID)
	return err
}

func (db *DB) ListAccounts() ([]*models.Account, error) {
	rows, err := db.conn.Query(`
		SELECT id, email, password, provider, status, credits_used, credits_total,
		       token, cookie, last_used_at, last_synced_at, error_message, created_at, updated_at
		FROM accounts ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		accounts = append(accounts, &acc)
	}
	return accounts, rows.Err()
}

func (db *DB) DeleteAccount(id int64) error {
	_, err := db.conn.Exec("DELETE FROM accounts WHERE id = ?", id)
	return err
}

func (db *DB) SaveRequestLog(log *models.RequestLog) error {
	log.CreatedAt = time.Now()
	_, err := db.conn.Exec(`
		INSERT INTO request_logs (model, provider, account_id, status, status_code, prompt_tokens,
		                          completion_tokens, total_tokens, latency_ms, error_message, ip_address, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, log.Model, log.Provider, log.AccountID, log.Status, log.StatusCode, log.PromptTokens,
		log.CompletionTokens, log.TotalTokens, log.LatencyMs, log.ErrorMessage, log.IPAddress, log.CreatedAt)
	return err
}

func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.conn.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (db *DB) SetSetting(key, value string) error {
	now := time.Now()
	_, err := db.conn.Exec(`
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value=?, updated_at=?
	`, key, value, now, value, now)
	return err
}
