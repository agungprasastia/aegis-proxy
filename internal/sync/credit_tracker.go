package sync

import (
	"fmt"

	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/models"
)

// CreditTracker tracks token usage against account credits.
type CreditTracker struct {
	db *database.DB
}

func NewCreditTracker(db *database.DB) *CreditTracker {
	return &CreditTracker{db: db}
}

func (ct *CreditTracker) EstimateCost(promptTokens, completionTokens int) float64 {
	return float64(promptTokens + completionTokens)
}

func (ct *CreditTracker) TrackUsage(account *models.Account, promptTokens, completionTokens int) error {
	if ct == nil {
		return fmt.Errorf("credit tracker is nil")
	}
	if ct.db == nil {
		return fmt.Errorf("database is not configured")
	}
	if account == nil {
		return fmt.Errorf("account is nil")
	}

	cost := ct.EstimateCost(promptTokens, completionTokens)
	account.CreditsUsed += cost

	if account.CreditsTotal > 0 && account.CreditsUsed >= account.CreditsTotal {
		account.CreditsUsed = account.CreditsTotal
		account.Status = models.StatusExhausted
	}

	if account.CreditsTotal <= 0 {
		account.Status = models.StatusExhausted
	}

	return ct.db.SaveAccount(account)
}
