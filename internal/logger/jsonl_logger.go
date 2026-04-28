package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	maxLogFileSize = 100 * 1024 * 1024 // 100MB
)

// RequestLogEntry represents a single request log entry
type RequestLogEntry struct {
	ID               string `json:"id"`
	Timestamp        string `json:"timestamp"`
	Model            string `json:"model"`
	Provider         string `json:"provider"`
	Status           string `json:"status"`
	StatusCode       int    `json:"status_code"`
	Latency          int64  `json:"latency_ns"`
	AccountEmail     string `json:"account_email"`
	Error            string `json:"error,omitempty"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
}

// JSONLLogger handles JSONL request logging
type JSONLLogger struct {
	filePath    string
	mu          sync.Mutex
	currentSize int64
}

// NewJSONLLogger creates a new JSONL logger
func NewJSONLLogger(filePath string) (*JSONLLogger, error) {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Get current file size if exists
	var currentSize int64
	if info, err := os.Stat(filePath); err == nil {
		currentSize = info.Size()
	}

	return &JSONLLogger{
		filePath:    filePath,
		currentSize: currentSize,
	}, nil
}

// Log appends a request log entry to the JSONL file
func (l *JSONLLogger) Log(entry RequestLogEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Generate ID if not provided
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	// Set timestamp if not provided
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	// Check if rotation is needed
	if l.currentSize >= maxLogFileSize {
		if err := l.rotate(); err != nil {
			return fmt.Errorf("failed to rotate log file: %w", err)
		}
	}

	// Marshal entry to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Append newline
	data = append(data, '\n')

	// Open file in append mode
	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	// Write entry
	n, err := f.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	// Update current size
	l.currentSize += int64(n)

	return nil
}

// rotate moves the current log file to .jsonl.1
func (l *JSONLLogger) rotate() error {
	// Close and rename current file
	rotatedPath := l.filePath + ".1"

	// Remove old rotated file if exists
	if _, err := os.Stat(rotatedPath); err == nil {
		if err := os.Remove(rotatedPath); err != nil {
			return fmt.Errorf("failed to remove old rotated file: %w", err)
		}
	}

	// Rename current file
	if err := os.Rename(l.filePath, rotatedPath); err != nil {
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	// Reset current size
	l.currentSize = 0

	return nil
}
