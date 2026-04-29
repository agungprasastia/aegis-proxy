package accounts

import (
	"context"
	"log"
	"time"

	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
)

// WarmupAccount sends a small test request to verify the account is working
func WarmupAccount(prov provider.Provider, account *models.Account, db *database.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &provider.ChatRequest{
		Model: "auto",
		Messages: []provider.ChatMessage{
			{
				Role:    "user",
				Content: "hi",
			},
		},
		MaxTokens: 1,
	}

	_, err := prov.SendChatCompletion(ctx, account, req)
	if err != nil {
		log.Printf("warmup failed email=%s provider=%s error=%v", account.Email, account.Provider, err)
		return err
	}

	log.Printf("warmup success email=%s provider=%s", account.Email, account.Provider)
	return nil
}
