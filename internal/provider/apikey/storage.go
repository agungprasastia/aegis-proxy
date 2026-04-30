package apikey

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"time"
)

func SaveAPIKey(db *sql.DB, provider, apiKey string) error {
	if provider == "" || apiKey == "" {
		return fmt.Errorf("provider and api key are required")
	}

	encrypted, err := encryptAPIKey(apiKey)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM api_keys WHERE provider = ?", provider); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO api_keys (provider, key, status, created_at) VALUES (?, ?, 'active', ?)", provider, encrypted, time.Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func GetAPIKey(db *sql.DB, provider string) (string, error) {
	var encrypted string
	err := db.QueryRow("SELECT key FROM api_keys WHERE provider = ? AND status = 'active' ORDER BY created_at DESC LIMIT 1", provider).Scan(&encrypted)
	if err != nil {
		return "", err
	}
	return decryptAPIKey(encrypted)
}

func DeleteAPIKey(db *sql.DB, provider string) error {
	_, err := db.Exec("DELETE FROM api_keys WHERE provider = ?", provider)
	return err
}

func ListAPIKeyProviders(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT DISTINCT provider FROM api_keys WHERE status = 'active' ORDER BY provider")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := []string{}
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

func encryptAPIKey(apiKey string) (string, error) {
	block, err := aes.NewCipher(encryptionKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(apiKey), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func decryptAPIKey(encrypted string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(encryptionKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted api key is invalid")
	}
	nonce := data[:gcm.NonceSize()]
	ciphertext := data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func encryptionKey() []byte {
	seed := os.Getenv("AEGIS_API_KEY_ENCRYPTION_KEY")
	if seed == "" {
		host, _ := os.Hostname()
		configDir, _ := os.UserConfigDir()
		seed = host + ":" + configDir
	}
	sum := sha256.Sum256([]byte(seed))
	return sum[:]
}
