package auth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ProviderCredentials represents credentials for a single provider
type ProviderCredentials struct {
	Success     bool                   `json:"success"`
	Provider    string                 `json:"provider"`
	Credentials map[string]interface{} `json:"credentials,omitempty"`
	Quota       map[string]interface{} `json:"quota,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// LoginResult contains credentials from all providers
type LoginResult struct {
	Kiro      *ProviderCredentials `json:"kiro"`
	CodeBuddy *ProviderCredentials `json:"codebuddy"`
	Wavespeed *ProviderCredentials `json:"wavespeed"`
	Canva     *ProviderCredentials `json:"canva"`
	YepAPI    *ProviderCredentials `json:"yepapi"`
}

// ProgressEvent represents a progress update from Python script
type ProgressEvent struct {
	Type     string `json:"type"`
	Provider string `json:"provider"`
	Step     string `json:"step"`
	Message  string `json:"message"`
}

// ErrorEvent represents an error from Python script
type ErrorEvent struct {
	Type     string `json:"type"`
	Provider string `json:"provider"`
	Error    string `json:"error"`
	Code     string `json:"code,omitempty"`
}

// ResultEvent represents the final result from Python script
type ResultEvent struct {
	Type      string                 `json:"type"`
	Kiro      map[string]interface{} `json:"kiro"`
	CodeBuddy map[string]interface{} `json:"codebuddy"`
	Wavespeed map[string]interface{} `json:"wavespeed"`
	Canva     map[string]interface{} `json:"canva"`
	YepAPI    map[string]interface{} `json:"yepapi"`
}

// RunLogin executes the Python login script and returns credentials for all providers
func RunLogin(email, password string) (*LoginResult, error) {
	// Create command: python auth/login.py --email X --password Y
	cmd := exec.Command("python", "auth/login.py", "--email", email, "--password", password)

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start the subprocess
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start python subprocess: %w", err)
	}

	// Create scanner to read line-by-line
	scanner := bufio.NewScanner(stdout)
	var result *LoginResult

	// Parse JSON output line-by-line
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try to parse as generic JSON to determine type
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Skip invalid JSON lines
			continue
		}

		eventType, ok := event["type"].(string)
		if !ok {
			continue
		}

		switch eventType {
		case "progress":
			// Parse progress event (optional: log it)
			var progress ProgressEvent
			if err := json.Unmarshal([]byte(line), &progress); err == nil {
				// Optional: log progress events
				// fmt.Printf("[%s] %s: %s\n", progress.Provider, progress.Step, progress.Message)
			}

		case "error":
			// Parse error event (optional: log it)
			var errEvent ErrorEvent
			if err := json.Unmarshal([]byte(line), &errEvent); err == nil {
				// Optional: log error events
				// fmt.Printf("[%s] ERROR: %s\n", errEvent.Provider, errEvent.Error)
			}

		case "result":
			// Parse final result
			var resultEvent ResultEvent
			if err := json.Unmarshal([]byte(line), &resultEvent); err != nil {
				return nil, fmt.Errorf("failed to parse result event: %w", err)
			}

			// Convert to LoginResult
			result = &LoginResult{
				Kiro:      mapToProviderCredentials(resultEvent.Kiro),
				CodeBuddy: mapToProviderCredentials(resultEvent.CodeBuddy),
				Wavespeed: mapToProviderCredentials(resultEvent.Wavespeed),
				Canva:     mapToProviderCredentials(resultEvent.Canva),
				YepAPI:    mapToProviderCredentials(resultEvent.YepAPI),
			}
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading subprocess output: %w", err)
	}

	// Wait for subprocess to complete
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("python subprocess failed: %w", err)
	}

	// Ensure we got a result
	if result == nil {
		return nil, fmt.Errorf("no result received from python script")
	}

	return result, nil
}

// mapToProviderCredentials converts a map to ProviderCredentials struct
func mapToProviderCredentials(data map[string]interface{}) *ProviderCredentials {
	if data == nil {
		return nil
	}

	creds := &ProviderCredentials{}

	// Extract success field
	if success, ok := data["success"].(bool); ok {
		creds.Success = success
	}

	// Extract provider field
	if provider, ok := data["provider"].(string); ok {
		creds.Provider = provider
	}

	// Extract error field
	if errMsg, ok := data["error"].(string); ok {
		creds.Error = errMsg
	}

	// Extract credentials field
	if credentials, ok := data["credentials"].(map[string]interface{}); ok {
		creds.Credentials = credentials
	}

	// Extract quota field
	if quota, ok := data["quota"].(map[string]interface{}); ok {
		creds.Quota = quota
	}

	return creds
}
