package auth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// CodexResult represents the result from Codex login
type CodexResult struct {
	Success      bool    `json:"success"`
	Email        string  `json:"email"`
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	IDToken      string  `json:"id_token"`
	UsedPercent  float64 `json:"used_percent"`
}

// RunCodexLogin executes the Codex login Python script and parses the result
func RunCodexLogin() (*CodexResult, error) {
	// Create command to run Python script
	cmd := exec.Command("python", "auth/codex_login.py")

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start codex login: %w", err)
	}

	// Create scanner to read line by line
	scanner := bufio.NewScanner(stdout)
	var result *CodexResult

	// Read output line by line
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse JSON line
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Skip non-JSON lines
			continue
		}

		// Check event type
		eventType, ok := event["type"].(string)
		if !ok {
			continue
		}

		// Handle different event types
		switch eventType {
		case "result":
			// Parse the result
			result = &CodexResult{
				Success:      getBool(event, "success"),
				Email:        getString(event, "email"),
				AccessToken:  getString(event, "access_token"),
				RefreshToken: getString(event, "refresh_token"),
				IDToken:      getString(event, "id_token"),
				UsedPercent:  getFloat(event, "used_percent"),
			}
			// Break out of loop once we have the result
			goto done

		case "error":
			// Error occurred
			errMsg := getString(event, "error")
			return nil, fmt.Errorf("codex login failed: %s", errMsg)

		case "progress":
			// Progress events - ignore for now
			// Could be logged if needed
			continue
		}
	}

done:
	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading output: %w", err)
	}

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("codex login process failed: %w", err)
	}

	// Check if we got a result
	if result == nil {
		return nil, fmt.Errorf("no result received from codex login")
	}

	// Validate result
	if !result.Success {
		return nil, fmt.Errorf("codex login was not successful")
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("no access token received")
	}

	return result, nil
}

// Helper functions to safely extract values from map

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return false
}

func getFloat(m map[string]interface{}, key string) float64 {
	if val, ok := m[key].(float64); ok {
		return val
	}
	return 0.0
}
