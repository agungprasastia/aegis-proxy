package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
}

// findAuthDir locates the auth directory with login.py
func findAuthDir() string {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	homeDir, _ := os.UserHomeDir()

	candidates := []string{
		filepath.Join(".", "auth"),
		filepath.Join(exeDir, "auth"),
		filepath.Join(exeDir, "..", "auth"),
		filepath.Join(homeDir, ".aegis-proxy", "auth"),
	}

	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "login.py")); err == nil {
			return dir
		}
	}
	return "auth" // fallback
}

// findVenvPython locates the Python executable inside the venv
func findVenvPython(authDir string) string {
	var venvPython string
	if runtime.GOOS == "windows" {
		venvPython = filepath.Join(authDir, ".venv", "Scripts", "python.exe")
	} else {
		venvPython = filepath.Join(authDir, ".venv", "bin", "python")
	}
	if _, err := os.Stat(venvPython); err == nil {
		return venvPython
	}
	return "python" // fallback to system python
}

// RunLogin executes the Python login script and returns credentials for all providers
func RunLogin(email, password string) (*LoginResult, error) {
	authDir := findAuthDir()
	pythonExe := findVenvPython(authDir)
	loginScript := filepath.Join(authDir, "login.py")

	cmd := exec.Command(pythonExe, loginScript, "--email", email, "--password", password)

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

// ProgressCallback is called for each progress event during login
type ProgressCallback func(ProgressEvent)

// LoginOptions holds configuration for the login process
type LoginOptions struct {
	Headless   bool
	Concurrent int
	Priority   string
	ProxyURL   string
	Provider   string // If set, only login to this specific provider
}

// RunLoginWithProgress executes login with progress callbacks for real-time logging
func RunLoginWithProgress(email, password string, onProgress ProgressCallback) (*LoginResult, error) {
	return RunLoginWithOptions(email, password, LoginOptions{
		Headless:   true,
		Concurrent: 2,
		Priority:   "standard",
	}, onProgress)
}

// RunLoginWithOptions executes login with full configuration.
// It accepts a context so the subprocess can be killed on cancellation.
func RunLoginWithOptions(email, password string, opts LoginOptions, onProgress ProgressCallback) (*LoginResult, error) {
	return RunLoginWithOptionsCtx(context.Background(), email, password, opts, onProgress)
}

// RunLoginWithOptionsCtx is like RunLoginWithOptions but accepts a context.
// When the context is cancelled, the Python subprocess is killed immediately.
func RunLoginWithOptionsCtx(ctx context.Context, email, password string, opts LoginOptions, onProgress ProgressCallback) (*LoginResult, error) {
	authDir := findAuthDir()
	pythonExe := findVenvPython(authDir)
	loginScript := filepath.Join(authDir, "login.py")

	args := []string{loginScript, "--email", email, "--password", password}
	if opts.Provider != "" {
		args = append(args, "--provider", opts.Provider)
	}

	// Use CommandContext so the subprocess is killed when ctx is cancelled
	cmd := exec.CommandContext(ctx, pythonExe, args...)

	// Set environment variables for Python script
	cmd.Env = append(os.Environ(),
		"BATCHER_ENABLE_CAMOUFOX=true",
		fmt.Sprintf("BATCHER_CAMOUFOX_HEADLESS=%v", opts.Headless),
		fmt.Sprintf("BATCHER_CONCURRENT=%d", opts.Concurrent),
		fmt.Sprintf("BATCHER_PRIORITY=%s", opts.Priority),
	)
	if opts.ProxyURL != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("BATCHER_PROXY_URL=%s", opts.ProxyURL))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start python subprocess: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	var result *LoginResult

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		eventType, ok := event["type"].(string)
		if !ok {
			continue
		}

		switch eventType {
		case "progress":
			var progress ProgressEvent
			if err := json.Unmarshal([]byte(line), &progress); err == nil {
				if onProgress != nil {
					onProgress(progress)
				}
			}

		case "error":
			var errEvent ErrorEvent
			if err := json.Unmarshal([]byte(line), &errEvent); err == nil {
				if onProgress != nil {
					onProgress(ProgressEvent{
						Type:     "error",
						Provider: errEvent.Provider,
						Message:  errEvent.Error,
					})
				}
			}

		case "result":
			var resultEvent ResultEvent
			if err := json.Unmarshal([]byte(line), &resultEvent); err != nil {
				return nil, fmt.Errorf("failed to parse result event: %w", err)
			}

			result = &LoginResult{
				Kiro:      mapToProviderCredentials(resultEvent.Kiro),
				CodeBuddy: mapToProviderCredentials(resultEvent.CodeBuddy),
				Wavespeed: mapToProviderCredentials(resultEvent.Wavespeed),
				Canva:     mapToProviderCredentials(resultEvent.Canva),
			}
		}
	}

	if err := scanner.Err(); err != nil {
		// If context was cancelled, don't treat as error
		if ctx.Err() != nil {
			return nil, fmt.Errorf("login cancelled")
		}
		return nil, fmt.Errorf("error reading subprocess output: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		// If context was cancelled, the process was killed — expected
		if ctx.Err() != nil {
			return nil, fmt.Errorf("login cancelled")
		}
		return nil, fmt.Errorf("python subprocess failed: %w", err)
	}

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
