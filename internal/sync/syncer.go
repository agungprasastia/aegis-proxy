package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
)

const (
	defaultSyncInterval = 5 * time.Minute
)

type AccountSyncer struct {
	db       *database.DB
	ticker   *time.Ticker
	done     chan struct{}
	interval time.Duration
}

func NewAccountSyncer(db *database.DB) *AccountSyncer {
	return &AccountSyncer{
		db:       db,
		interval: defaultSyncInterval,
		done:     make(chan struct{}),
	}
}

func (s *AccountSyncer) Start() {
	s.ticker = time.NewTicker(s.interval)
	go s.run()
	log.Printf("[Syncer] Started with interval: %v", s.interval)
}

func (s *AccountSyncer) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.done)
	log.Println("[Syncer] Stopped")
}

func (s *AccountSyncer) run() {
	for {
		select {
		case <-s.done:
			return
		case <-s.ticker.C:
			s.syncAllAccounts()
		}
	}
}

func (s *AccountSyncer) Sync() error {
	return s.syncAllAccounts()
}

func (s *AccountSyncer) syncAllAccounts() error {
	accounts, err := s.db.ListAccounts()
	if err != nil {
		log.Printf("[Syncer] Failed to list accounts: %v", err)
		return err
	}

	log.Printf("[Syncer] Syncing %d accounts...", len(accounts))
	for _, account := range accounts {
		if err := s.syncAccount(account); err != nil {
			log.Printf("[Syncer] Failed to sync account %s (%s): %v", account.Email, account.Provider, err)
		}
	}
	log.Println("[Syncer] Sync completed")
	return nil
}

func (s *AccountSyncer) syncAccount(account *models.Account) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch account.Provider {
	case models.ProviderKiro:
		return s.syncKiroAccount(ctx, account)
	case models.ProviderCodeBuddy:
		return s.syncCodeBuddyAccount(ctx, account)
	case models.ProviderWindsurf, models.ProviderCanva, models.ProviderCodex, models.ProviderWavespeed:
		return s.syncGenericAccount(ctx, account)
	default:
		return fmt.Errorf("unknown provider: %s", account.Provider)
	}
}

func (s *AccountSyncer) syncKiroAccount(ctx context.Context, account *models.Account) error {
	p, ok := provider.ProviderRegistry[models.ProviderKiro]
	if !ok {
		return fmt.Errorf("kiro provider not found")
	}

	used, total, err := p.GetCredits(ctx, account)
	if err != nil {
		account.Status = models.StatusError
		account.ErrorMessage = err.Error()
		now := time.Now()
		account.LastSyncedAt = &now
		return s.db.SaveAccount(account)
	}

	account.CreditsUsed = used
	account.CreditsTotal = total
	if used >= total {
		account.Status = models.StatusExhausted
	} else {
		account.Status = models.StatusActive
	}
	account.ErrorMessage = ""
	now := time.Now()
	account.LastSyncedAt = &now

	return s.db.SaveAccount(account)
}

func (s *AccountSyncer) syncCodeBuddyAccount(ctx context.Context, account *models.Account) error {
	p, ok := provider.ProviderRegistry[models.ProviderCodeBuddy]
	if !ok {
		return fmt.Errorf("codebuddy provider not found")
	}

	if err := p.ValidateAccount(ctx, account); err != nil {
		account.Status = models.StatusError
		account.ErrorMessage = err.Error()
		now := time.Now()
		account.LastSyncedAt = &now
		return s.db.SaveAccount(account)
	}

	account.Status = models.StatusActive
	account.ErrorMessage = ""
	now := time.Now()
	account.LastSyncedAt = &now

	return s.db.SaveAccount(account)
}

func (s *AccountSyncer) syncGenericAccount(ctx context.Context, account *models.Account) error {
	p, ok := provider.ProviderRegistry[account.Provider]
	if !ok {
		return fmt.Errorf("provider %s not found", account.Provider)
	}

	if err := p.ValidateAccount(ctx, account); err != nil {
		account.Status = models.StatusError
		account.ErrorMessage = err.Error()
		now := time.Now()
		account.LastSyncedAt = &now
		return s.db.SaveAccount(account)
	}

	account.Status = models.StatusActive
	account.ErrorMessage = ""
	now := time.Now()
	account.LastSyncedAt = &now

	return s.db.SaveAccount(account)
}

type kiroTokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ProfileARN   string `json:"profile_arn"`
	ExpiresAt    string `json:"expires_at"`
	ExpiresIn    string `json:"expires_in"`
}

func (s *AccountSyncer) refreshKiroToken(ctx context.Context, account *models.Account) error {
	var tokenData kiroTokenData
	if err := json.Unmarshal([]byte(account.Token), &tokenData); err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
	}

	if tokenData.RefreshToken == "" {
		return fmt.Errorf("refresh_token not found")
	}

	return nil
}
