package logger

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/models"
)

type RequestLogger struct {
	db *database.DB
}

type Stats struct {
	TotalRequests  int     `json:"total_requests"`
	SuccessCount   int     `json:"success_count"`
	FailCount      int     `json:"fail_count"`
	SuccessRate    float64 `json:"success_rate"`
	UptimeSeconds  int64   `json:"uptime_seconds"`
}

type TokenUsage struct {
	Total      int64         `json:"total"`
	Prompt     int64         `json:"prompt"`
	Completion int64         `json:"completion"`
	ByModel    []ModelUsage  `json:"by_model"`
	Hourly     []HourlyUsage `json:"hourly"`
}

type ModelUsage struct {
	Model    string `json:"model"`
	Tokens   int64  `json:"tokens"`
	Requests int    `json:"requests"`
}

type HourlyUsage struct {
	Hour   string `json:"hour"`
	Tokens int64  `json:"tokens"`
}

func NewRequestLogger(db *database.DB) *RequestLogger {
	return &RequestLogger{db: db}
}

func (rl *RequestLogger) Log(log *models.RequestLog) error {
	log.CreatedAt = time.Now()
	_, err := rl.db.Exec(`
		INSERT INTO request_logs (model, provider, account_id, status, status_code, prompt_tokens,
		                          completion_tokens, total_tokens, latency_ms, error_message, ip_address, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, log.Model, log.Provider, log.AccountID, log.Status, log.StatusCode, log.PromptTokens,
		log.CompletionTokens, log.TotalTokens, log.LatencyMs, log.ErrorMessage, log.IPAddress, log.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert request log: %w", err)
	}
	return nil
}

func (rl *RequestLogger) GetLogs(page, limit int, filters map[string]string) ([]models.RequestLog, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}

	offset := (page - 1) * limit

	query := `
		SELECT id, model, provider, account_id, status, status_code, prompt_tokens,
		       completion_tokens, total_tokens, latency_ms, error_message, ip_address, created_at
		FROM request_logs
	`
	countQuery := "SELECT COUNT(*) FROM request_logs"
	args := []interface{}{}
	whereClauses := []string{}

	if status, ok := filters["status"]; ok && status != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, status)
	}

	if model, ok := filters["model"]; ok && model != "" {
		whereClauses = append(whereClauses, "model = ?")
		args = append(args, model)
	}

	if provider, ok := filters["provider"]; ok && provider != "" {
		whereClauses = append(whereClauses, "provider = ?")
		args = append(args, provider)
	}

	if len(whereClauses) > 0 {
		whereClause := " WHERE " + strings.Join(whereClauses, " AND ")
		query += whereClause
		countQuery += whereClause
	}

	var total int
	if err := rl.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := rl.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var logs []models.RequestLog
	for rows.Next() {
		var log models.RequestLog
		if err := rows.Scan(
			&log.ID, &log.Model, &log.Provider, &log.AccountID, &log.Status, &log.StatusCode,
			&log.PromptTokens, &log.CompletionTokens, &log.TotalTokens, &log.LatencyMs,
			&log.ErrorMessage, &log.IPAddress, &log.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, total, rows.Err()
}

func (rl *RequestLogger) GetStats(timeframe string) (*Stats, error) {
	since := rl.getTimeframeSince(timeframe)

	query := `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success,
			SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as fail
		FROM request_logs
	`
	args := []interface{}{}

	if since != nil {
		query += " WHERE created_at >= ?"
		args = append(args, since)
	}

	var stats Stats
	var success, fail sql.NullInt64

	err := rl.db.QueryRow(query, args...).Scan(&stats.TotalRequests, &success, &fail)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	if success.Valid {
		stats.SuccessCount = int(success.Int64)
	}
	if fail.Valid {
		stats.FailCount = int(fail.Int64)
	}

	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalRequests) * 100
	}

	if since != nil {
		stats.UptimeSeconds = int64(time.Since(*since).Seconds())
	}

	return &stats, nil
}

func (rl *RequestLogger) GetTokenUsage(timeframe string) (*TokenUsage, error) {
	since := rl.getTimeframeSince(timeframe)

	query := `
		SELECT 
			COALESCE(SUM(total_tokens), 0) as total,
			COALESCE(SUM(prompt_tokens), 0) as prompt,
			COALESCE(SUM(completion_tokens), 0) as completion
		FROM request_logs
	`
	args := []interface{}{}

	if since != nil {
		query += " WHERE created_at >= ?"
		args = append(args, since)
	}

	var usage TokenUsage
	err := rl.db.QueryRow(query, args...).Scan(&usage.Total, &usage.Prompt, &usage.Completion)
	if err != nil {
		return nil, fmt.Errorf("failed to get token usage: %w", err)
	}

	modelQuery := `
		SELECT model, COALESCE(SUM(total_tokens), 0) as tokens, COUNT(*) as requests
		FROM request_logs
	`
	modelArgs := []interface{}{}

	if since != nil {
		modelQuery += " WHERE created_at >= ?"
		modelArgs = append(modelArgs, since)
	}

	modelQuery += " GROUP BY model ORDER BY tokens DESC"

	rows, err := rl.db.Query(modelQuery, modelArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get model usage: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var mu ModelUsage
		if err := rows.Scan(&mu.Model, &mu.Tokens, &mu.Requests); err != nil {
			return nil, fmt.Errorf("failed to scan model usage: %w", err)
		}
		usage.ByModel = append(usage.ByModel, mu)
	}

	hourlyQuery := `
		SELECT strftime('%Y-%m-%d %H:00', created_at) as hour, COALESCE(SUM(total_tokens), 0) as tokens
		FROM request_logs
	`
	hourlyArgs := []interface{}{}

	if since != nil {
		hourlyQuery += " WHERE created_at >= ?"
		hourlyArgs = append(hourlyArgs, since)
	}

	hourlyQuery += " GROUP BY hour ORDER BY hour DESC LIMIT 24"

	hourlyRows, err := rl.db.Query(hourlyQuery, hourlyArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get hourly usage: %w", err)
	}
	defer hourlyRows.Close()

	for hourlyRows.Next() {
		var hu HourlyUsage
		if err := hourlyRows.Scan(&hu.Hour, &hu.Tokens); err != nil {
			return nil, fmt.Errorf("failed to scan hourly usage: %w", err)
		}
		usage.Hourly = append(usage.Hourly, hu)
	}

	return &usage, nil
}

func (rl *RequestLogger) GetRecentByModel(timeframe string) ([]ModelUsage, error) {
	since := rl.getTimeframeSince(timeframe)

	query := `
		SELECT model, COALESCE(SUM(total_tokens), 0) as tokens, COUNT(*) as requests
		FROM request_logs
	`
	args := []interface{}{}

	if since != nil {
		query += " WHERE created_at >= ?"
		args = append(args, since)
	}

	query += " GROUP BY model ORDER BY requests DESC"

	rows, err := rl.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent by model: %w", err)
	}
	defer rows.Close()

	var results []ModelUsage
	for rows.Next() {
		var mu ModelUsage
		if err := rows.Scan(&mu.Model, &mu.Tokens, &mu.Requests); err != nil {
			return nil, fmt.Errorf("failed to scan model usage: %w", err)
		}
		results = append(results, mu)
	}

	return results, rows.Err()
}

func (rl *RequestLogger) getTimeframeSince(timeframe string) *time.Time {
	now := time.Now()
	var since time.Time

	switch timeframe {
	case "1d":
		since = now.Add(-24 * time.Hour)
	case "7d":
		since = now.Add(-7 * 24 * time.Hour)
	case "30d":
		since = now.Add(-30 * 24 * time.Hour)
	case "all":
		return nil
	default:
		since = now.Add(-24 * time.Hour)
	}

	return &since
}
