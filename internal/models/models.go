package models

import (
	"time"
)

const (
	ProviderKiro      = "kiro"
	ProviderCodeBuddy = "codebuddy"
	ProviderWindsurf  = "windsurf"
	ProviderCanva     = "canva"
	ProviderYepAPI    = "yepapi"
	ProviderCodex     = "codex"
	ProviderWavespeed = "wavespeed"
	ProviderDeepSeek  = "deepseek"
	ProviderGroq      = "groq"
	ProviderGLM       = "glm"
	ProviderMiniMax   = "minimax"
	ProviderMistral   = "mistral"
	ProviderXAI       = "xai"
)

const (
	StatusActive    = "active"
	StatusExhausted = "exhausted"
	StatusBanned    = "banned"
	StatusError     = "error"
	StatusPending   = "pending"
)

type Account struct {
	ID           int64      `json:"id" db:"id"`
	Email        string     `json:"email" db:"email"`
	Password     string     `json:"password" db:"password"`
	Provider     string     `json:"provider" db:"provider"`
	Status       string     `json:"status" db:"status"`
	CreditsUsed  float64    `json:"credits_used" db:"credits_used"`
	CreditsTotal float64    `json:"credits_total" db:"credits_total"`
	Token        string     `json:"token" db:"token"`
	Cookie       string     `json:"cookie" db:"cookie"`
	LastUsedAt   *time.Time `json:"last_used_at" db:"last_used_at"`
	LastSyncedAt *time.Time `json:"last_synced_at" db:"last_synced_at"`
	ErrorMessage string     `json:"error_message" db:"error_message"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

type RequestLog struct {
	ID               int64     `json:"id" db:"id"`
	Model            string    `json:"model" db:"model"`
	Provider         string    `json:"provider" db:"provider"`
	AccountID        *int64    `json:"account_id" db:"account_id"`
	Status           string    `json:"status" db:"status"`
	StatusCode       int       `json:"status_code" db:"status_code"`
	PromptTokens     int       `json:"prompt_tokens" db:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens" db:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens" db:"total_tokens"`
	LatencyMs        int       `json:"latency_ms" db:"latency_ms"`
	ErrorMessage     string    `json:"error_message" db:"error_message"`
	IPAddress        string    `json:"ip_address" db:"ip_address"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

type Setting struct {
	Key       string    `json:"key" db:"key"`
	Value     string    `json:"value" db:"value"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type ProxyEntry struct {
	ID           int64      `json:"id" db:"id"`
	URL          string     `json:"url" db:"url"`
	Type         string     `json:"type" db:"type"`
	Status       string     `json:"status" db:"status"`
	LatencyMs    int        `json:"latency_ms" db:"latency_ms"`
	LastTestedAt *time.Time `json:"last_tested_at" db:"last_tested_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}
