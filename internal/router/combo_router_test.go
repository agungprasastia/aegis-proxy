package router

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/aegis-proxy/aegis/internal/accounts"
	"github.com/aegis-proxy/aegis/internal/combo"
	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
)

func TestCombo429FallsBackToSecondary(t *testing.T) {
	r, primary, secondary := newComboTestRouter(t)
	combo := createTestCombo(t, r, []models.ComboTarget{{Provider: models.ProviderKiro, Model: "claude-sonnet-4", Priority: 1}, {Provider: models.ProviderCodeBuddy, Model: "gpt-5.2", Priority: 2}})
	primary.errs = []error{NewComboError(http.StatusTooManyRequests, "rate limit exceeded", nil)}
	secondary.responses = []*provider.ChatResponse{chatResp("secondary ok")}

	resp, err := r.ExecuteWithFallback(WithClientID(context.Background(), "client-a"), combo, &models.NormalizedRequest{Model: combo.Name, Messages: []models.NormalizedMessage{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("ExecuteWithFallback() error = %v", err)
	}
	if resp.Content != "secondary ok" || primary.calls != 1 || secondary.calls != 1 {
		t.Fatalf("fallback result content=%q primary=%d secondary=%d", resp.Content, primary.calls, secondary.calls)
	}
}

func TestComboMalformedRequestDoesNotFallback(t *testing.T) {
	r, primary, secondary := newComboTestRouter(t)
	combo := createTestCombo(t, r, []models.ComboTarget{{Provider: models.ProviderKiro, Model: "claude-sonnet-4", Priority: 1}, {Provider: models.ProviderCodeBuddy, Model: "gpt-5.2", Priority: 2}})
	primary.errs = []error{NewComboError(http.StatusBadRequest, "malformed request", nil)}
	secondary.responses = []*provider.ChatResponse{chatResp("should not run")}

	_, err := r.ExecuteWithFallback(WithClientID(context.Background(), "client-b"), combo, &models.NormalizedRequest{Model: combo.Name, Messages: []models.NormalizedMessage{{Role: "user", Content: "bad"}}})
	if err == nil {
		t.Fatalf("ExecuteWithFallback() expected error")
	}
	if primary.calls != 1 || secondary.calls != 0 {
		t.Fatalf("calls primary=%d secondary=%d, want 1/0", primary.calls, secondary.calls)
	}
}

func TestComboFiniteRetryBudget(t *testing.T) {
	r, primary, secondary := newComboTestRouter(t)
	combo := createTestCombo(t, r, []models.ComboTarget{{Provider: models.ProviderKiro, Model: "claude-sonnet-4", Priority: 1}, {Provider: models.ProviderCodeBuddy, Model: "gpt-5.2", Priority: 2}})
	primary.errs = []error{NewComboError(http.StatusTooManyRequests, "rate limit exceeded", nil)}
	secondary.errs = []error{fmt.Errorf("server error: status 500")}

	_, err := r.ExecuteWithFallback(WithClientID(context.Background(), "client-c"), combo, &models.NormalizedRequest{Model: combo.Name, Messages: []models.NormalizedMessage{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatalf("ExecuteWithFallback() expected error")
	}
	if primary.calls != 1 || secondary.calls != 1 {
		t.Fatalf("calls primary=%d secondary=%d, want one per target", primary.calls, secondary.calls)
	}
}

func TestComboStickySessionsStillWork(t *testing.T) {
	r, primary, _ := newComboTestRouter(t)
	combo := createTestCombo(t, r, []models.ComboTarget{{Provider: models.ProviderKiro, Model: "claude-sonnet-4", Priority: 1}})
	primary.responses = []*provider.ChatResponse{chatResp("first"), chatResp("second")}

	for i := 0; i < 2; i++ {
		_, err := r.ExecuteWithFallback(WithClientID(context.Background(), "sticky-client"), combo, &models.NormalizedRequest{Model: combo.Name, Messages: []models.NormalizedMessage{{Role: "user", Content: "hi"}}})
		if err != nil {
			t.Fatalf("ExecuteWithFallback(%d) error = %v", i, err)
		}
	}
	if len(primary.accountIDs) != 2 || primary.accountIDs[0] != primary.accountIDs[1] {
		t.Fatalf("sticky account IDs = %#v", primary.accountIDs)
	}
}

func newComboTestRouter(t *testing.T) (*Router, *fakeProvider, *fakeProvider) {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "combo-router.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	am := accounts.NewAccountManager(db)
	insertAccount(t, db, models.ProviderKiro, "p1@example.com")
	insertAccount(t, db, models.ProviderKiro, "p2@example.com")
	insertAccount(t, db, models.ProviderCodeBuddy, "s1@example.com")
	primary := &fakeProvider{name: "primary"}
	secondary := &fakeProvider{name: "secondary"}
	r := NewRouter(map[string]provider.Provider{models.ProviderKiro: primary, models.ProviderCodeBuddy: secondary}, am)
	r.SetComboService(combo.NewService(db))
	return r, primary, secondary
}

func createTestCombo(t *testing.T, r *Router, targets []models.ComboTarget) *models.Combo {
	t.Helper()
	c := &models.Combo{Name: fmt.Sprintf("combo-%d", time.Now().UnixNano()), Targets: targets}
	service := r.comboService.(*combo.Service)
	if err := service.Create(c); err != nil {
		t.Fatalf("Create(combo) error = %v", err)
	}
	return c
}

func insertAccount(t *testing.T, db *database.DB, providerName, email string) {
	t.Helper()
	now := time.Now()
	if _, err := db.Exec(`INSERT INTO accounts (email, password, provider, status, token, cookie, error_message, created_at, updated_at) VALUES (?, 'pw', ?, ?, 'token', '', '', ?, ?)`, email, providerName, models.StatusActive, now, now); err != nil {
		t.Fatalf("insert account error = %v", err)
	}
}

type fakeProvider struct {
	name       string
	calls      int
	errs       []error
	responses  []*provider.ChatResponse
	accountIDs []int64
}

func (p *fakeProvider) Name() string { return p.name }
func (p *fakeProvider) Tier() string { return "test" }
func (p *fakeProvider) SendChatCompletion(ctx context.Context, account *models.Account, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	p.calls++
	p.accountIDs = append(p.accountIDs, account.ID)
	if len(p.errs) >= p.calls && p.errs[p.calls-1] != nil {
		return nil, p.errs[p.calls-1]
	}
	if len(p.responses) >= p.calls && p.responses[p.calls-1] != nil {
		return p.responses[p.calls-1], nil
	}
	return chatResp("ok"), nil
}
func (p *fakeProvider) SendChatCompletionStream(context.Context, *models.Account, *provider.ChatRequest, http.ResponseWriter) error {
	return nil
}
func (p *fakeProvider) ValidateAccount(context.Context, *models.Account) error { return nil }
func (p *fakeProvider) GetCredits(context.Context, *models.Account) (float64, float64, error) {
	return 0, 0, nil
}
func (p *fakeProvider) SupportsModel(string) bool      { return true }
func (p *fakeProvider) MapModel(modelID string) string { return modelID }

func chatResp(content string) *provider.ChatResponse {
	return &provider.ChatResponse{ID: "id", Object: "chat.completion", Created: time.Now().Unix(), Choices: []provider.Choice{{Index: 0, Message: provider.ChatMessage{Role: "assistant", Content: content}, FinishReason: string(models.FinishReasonStop)}}}
}
