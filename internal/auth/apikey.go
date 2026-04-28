package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/aegis-proxy/aegis/internal/database"
)

const (
	APIKeyPrefix = "sk-aegis-"
	APIKeyLength = 32
)

func GenerateAPIKey() string {
	bytes := make([]byte, APIKeyLength)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("failed to generate random bytes: %v", err))
	}
	return APIKeyPrefix + hex.EncodeToString(bytes)
}

func ValidateAPIKey(db *database.DB, key string) bool {
	storedKey, err := GetAPIKey(db)
	if err != nil {
		return false
	}
	return storedKey == key
}

func GetAPIKey(db *database.DB) (string, error) {
	value, err := db.GetSetting("api_key")
	if err != nil {
		return "", fmt.Errorf("failed to get api key: %w", err)
	}
	return value, nil
}

func SetAPIKey(db *database.DB, key string) error {
	if err := db.SetSetting("api_key", key); err != nil {
		return fmt.Errorf("failed to set api key: %w", err)
	}
	return nil
}

func RegenerateAPIKey(db *database.DB) (string, error) {
	key := GenerateAPIKey()
	if err := SetAPIKey(db, key); err != nil {
		return "", err
	}
	return key, nil
}

func EnsureAPIKey(db *database.DB) (string, error) {
	key, err := GetAPIKey(db)
	if err != nil {
		return "", err
	}

	if key == "" {
		return RegenerateAPIKey(db)
	}

	return key, nil
}
