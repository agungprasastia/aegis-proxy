package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
)

const comboRequestTimeout = 30 * time.Second

type Combo = models.Combo
type NormalizedRequest = models.NormalizedRequest
type NormalizedResponse = models.NormalizedResponse

type ProviderTarget struct {
	Provider     provider.Provider
	Account      *models.Account
	ProviderName string
	Model        string
}

type ComboError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e *ComboError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return http.StatusText(e.StatusCode)
}

func (e *ComboError) Unwrap() error { return e.Err }

func NewComboError(statusCode int, message string, err error) *ComboError {
	return &ComboError{StatusCode: statusCode, Message: message, Err: err}
}

func ResolveCombo(ctx context.Context, comboName string) ([]ProviderTarget, error) {
	return nil, fmt.Errorf("combo resolver is not configured for %q", comboName)
}

func ExecuteWithFallback(ctx context.Context, combo *Combo, req *NormalizedRequest) (*NormalizedResponse, error) {
	return nil, fmt.Errorf("combo executor is not configured for %q", comboName(combo))
}

func (r *Router) ResolveCombo(ctx context.Context, comboName string) ([]ProviderTarget, error) {
	combo, err := r.GetCombo(ctx, comboName)
	if err != nil {
		return nil, err
	}
	return r.resolveComboTargets(ctx, combo)
}

func (r *Router) GetCombo(ctx context.Context, comboName string) (*Combo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.comboService == nil {
		return nil, fmt.Errorf("combo service not configured")
	}
	combos, err := r.comboService.List()
	if err != nil {
		return nil, fmt.Errorf("list combos: %w", err)
	}
	for _, combo := range combos {
		if combo.Name == comboName {
			return combo, nil
		}
	}
	return nil, fmt.Errorf("combo not found: %s", comboName)
}

func (r *Router) ExecuteWithFallback(ctx context.Context, combo *Combo, req *NormalizedRequest) (*NormalizedResponse, error) {
	if combo == nil {
		return nil, NewComboError(http.StatusBadRequest, "combo is nil", nil)
	}
	if req == nil {
		return nil, NewComboError(http.StatusBadRequest, "malformed request: request is nil", nil)
	}
	if req.Stream {
		return nil, NewComboError(http.StatusBadRequest, "streaming combo fallback is not supported", nil)
	}

	targets, err := r.resolveComboTargets(ctx, combo)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("combo target chain is empty")
	}

	clientID, _ := ctx.Value(clientIDContextKey{}).(string)
	var lastErr error
	for i, target := range targets {
		attemptReq := *req
		attemptReq.Model = target.Model

		account := target.Account
		if account == nil && clientID != "" {
			account, err = r.accountMgr.GetSticky(target.ProviderName, clientID)
		} else if account == nil {
			account, err = r.accountMgr.GetAvailable(target.ProviderName)
		}
		if err != nil {
			return nil, fmt.Errorf("no available accounts for provider %s: %w", target.ProviderName, err)
		}

		attemptCtx, cancel := context.WithTimeout(ctx, comboRequestTimeout)
		resp, err := target.Provider.SendChatCompletion(attemptCtx, account, normalizedToChatRequest(&attemptReq))
		cancel()
		if err == nil {
			_ = r.accountMgr.MarkUsed(account.ID)
			return chatResponseToNormalized(resp), nil
		}

		lastErr = err
		if shouldMarkAccountError(err) {
			_ = r.accountMgr.MarkError(account.ID, err.Error())
		}
		if !isFallbackError(err) || i == len(targets)-1 {
			return nil, err
		}
	}

	return nil, fmt.Errorf("combo fallback exhausted: %w", lastErr)
}

func (r *Router) resolveComboTargets(ctx context.Context, combo *Combo) ([]ProviderTarget, error) {
	var targets []ProviderTarget
	if err := r.appendComboTargets(ctx, combo, map[string]struct{}{}, &targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func (r *Router) appendComboTargets(ctx context.Context, combo *Combo, visiting map[string]struct{}, out *[]ProviderTarget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if combo == nil {
		return fmt.Errorf("combo is nil")
	}
	if _, ok := visiting[combo.Name]; ok {
		return fmt.Errorf("recursive combo target: %s", combo.Name)
	}
	visiting[combo.Name] = struct{}{}
	defer delete(visiting, combo.Name)

	for _, target := range combo.Targets {
		if target.Provider == "combo" {
			child, err := r.GetCombo(ctx, target.Model)
			if err != nil {
				return err
			}
			if err := r.appendComboTargets(ctx, child, visiting, out); err != nil {
				return err
			}
			continue
		}

		r.mu.RLock()
		prov, ok := r.providers[target.Provider]
		r.mu.RUnlock()
		if !ok {
			return fmt.Errorf("provider not found: %s", target.Provider)
		}
		*out = append(*out, ProviderTarget{Provider: prov, ProviderName: target.Provider, Model: target.Model})
	}
	return nil
}

func isFallbackError(err error) bool {
	if err == nil {
		return false
	}
	var comboErr *ComboError
	if errors.As(err, &comboErr) {
		if comboErr.StatusCode == http.StatusTooManyRequests || comboErr.StatusCode >= 500 {
			return true
		}
		if comboErr.StatusCode >= 400 && comboErr.StatusCode < 500 {
			msg := strings.ToLower(comboErr.Error())
			return strings.Contains(msg, "expired") || strings.Contains(msg, "quota exhausted")
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "status 429") ||
		strings.Contains(msg, "server error") ||
		strings.Contains(msg, "status 5") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "expired token") ||
		strings.Contains(msg, "expired api_key") ||
		strings.Contains(msg, "quota exhausted")
}

func shouldMarkAccountError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "authentication") || strings.Contains(msg, "expired") || strings.Contains(msg, "quota exhausted")
}

func normalizedToChatRequest(req *NormalizedRequest) *provider.ChatRequest {
	messages := make([]provider.ChatMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = provider.ChatMessage{Role: msg.Role, Content: msg.Content}
	}
	return &provider.ChatRequest{Model: req.Model, Messages: messages, Stream: req.Stream, MaxTokens: req.MaxTokens, Temperature: req.Temperature}
}

func chatResponseToNormalized(resp *provider.ChatResponse) *NormalizedResponse {
	if resp == nil {
		return &NormalizedResponse{}
	}
	out := &NormalizedResponse{Usage: models.NormalizedUsage{PromptTokens: resp.Usage.PromptTokens, CompletionTokens: resp.Usage.CompletionTokens, TotalTokens: resp.Usage.TotalTokens}}
	if len(resp.Choices) > 0 {
		out.Content = resp.Choices[0].Message.Content
		out.FinishReason = models.FinishReason(resp.Choices[0].FinishReason)
	}
	return out
}

func comboName(combo *Combo) string {
	if combo == nil {
		return ""
	}
	return combo.Name
}

type clientIDContextKey struct{}

func WithClientID(ctx context.Context, clientID string) context.Context {
	return context.WithValue(ctx, clientIDContextKey{}, clientID)
}
