package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/aegis-proxy/aegis/internal/accounts"
	"github.com/aegis-proxy/aegis/internal/api"
	"github.com/aegis-proxy/aegis/internal/auth"
	"github.com/aegis-proxy/aegis/internal/config"
	"github.com/aegis-proxy/aegis/internal/database"
	"github.com/aegis-proxy/aegis/internal/dashboard"
	"github.com/aegis-proxy/aegis/internal/logger"
	"github.com/aegis-proxy/aegis/internal/mitm"
	"github.com/aegis-proxy/aegis/internal/models"
	"github.com/aegis-proxy/aegis/internal/provider"
	"github.com/aegis-proxy/aegis/internal/proxy"
	"github.com/aegis-proxy/aegis/internal/proxypool"
	"golang.org/x/crypto/bcrypt"

	// Import all providers to register them
	_ "github.com/aegis-proxy/aegis/internal/provider/canva"
	_ "github.com/aegis-proxy/aegis/internal/provider/codebuddy"
	_ "github.com/aegis-proxy/aegis/internal/provider/codex"
	_ "github.com/aegis-proxy/aegis/internal/provider/kiro"
	_ "github.com/aegis-proxy/aegis/internal/provider/wavespeed"
	_ "github.com/aegis-proxy/aegis/internal/provider/windsurf"
	_ "github.com/aegis-proxy/aegis/internal/provider/yepapi"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		startServers()
		return
	}

	command := args[0]

	switch command {
	case "start":
		if len(args) > 1 && args[1] == "--host" && len(args) > 2 {
			startWithHost(args[2])
		} else {
			startServers()
		}
	case "stop":
		stopServer()
	case "status":
		showStatus()
	case "accounts":
		handleAccountsCommand(args[1:])
	case "apikey":
		handleAPIKeyCommand(args[1:])
	case "models":
		listModels()
	case "expose":
		handleExposeCommand(args[1:])
	case "mitm":
		handleMITMCommand(args[1:])
	case "setup":
		setupPythonAuth()
	case "version":
		showVersion()
	case "help":
		showHelp()
	default:
		fmt.Printf("%sUnknown command: %s%s\n", colorRed, command, colorReset)
		showHelp()
		os.Exit(1)
	}
}

func startServers() {
	printBanner()

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sError loading config: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	db, err := database.InitDB(cfg.DBPath)
	if err != nil {
		fmt.Printf("%sError initializing database: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	defer db.Close()

	apiKey, err := auth.EnsureAPIKey(db)
	if err != nil {
		fmt.Printf("%sError ensuring API key: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	cfg.SetAPIKey(apiKey)

	// Auto-migrate plaintext password to bcrypt hash
	if cfg.DashboardPassword != "" && cfg.DashboardPasswordHash == "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.DashboardPassword), 10)
		if err != nil {
			fmt.Printf("%sError hashing password: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		cfg.DashboardPasswordHash = string(hash)
		cfg.DashboardPassword = "" // Clear plaintext
		if err := cfg.Save(); err != nil {
			fmt.Printf("%sWarning: Could not save password hash: %v%s\n", colorYellow, err, colorReset)
		} else {
			fmt.Printf("%s✓ Migrated dashboard password to bcrypt hash%s\n", colorGreen, colorReset)
		}
	}

	// Set default password if none exists
	if cfg.DashboardPassword == "" && cfg.DashboardPasswordHash == "" {
		hash, err := bcrypt.GenerateFromPassword([]byte("admin"), 10)
		if err != nil {
			fmt.Printf("%sError hashing default password: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		cfg.DashboardPasswordHash = string(hash)
		if err := cfg.Save(); err != nil {
			fmt.Printf("%sWarning: Could not save default password: %v%s\n", colorYellow, err, colorReset)
		}
	}

	am := accounts.NewAccountManager(db)
	rl := logger.NewRequestLogger(db)

	pool, poolConfig, err := proxypool.LoadProxyPool(cfg.DataDir)
	if err != nil {
		fmt.Printf("%sWarning: Could not load proxy pool: %v%s\n", colorYellow, err, colorReset)
		pool = proxypool.NewProxyPool()
		poolConfig = &proxypool.ProxyPoolConfig{}
	}

	proxyServer := proxy.NewProxyServer(cfg, am, rl, pool)
	apiServer := api.NewAPIServer(db, am, rl, cfg, pool, poolConfig)
	dashboardServer := dashboard.NewDashboardServer(cfg, apiServer)

	errChan := make(chan error, 2)

	go func() {
		fmt.Printf("%s→ Proxy server starting on %s%s\n", colorGreen, cfg.ProxyAddr(), colorReset)
		if err := proxyServer.Start(); err != nil {
			errChan <- fmt.Errorf("proxy server error: %w", err)
		}
	}()

	go func() {
		fmt.Printf("%s→ Dashboard server starting on %s%s\n", colorCyan, cfg.DashboardAddr(), colorReset)
		if err := dashboardServer.Start(); err != nil {
			errChan <- fmt.Errorf("dashboard server error: %w", err)
		}
	}()

	fmt.Printf("\n%s✓ Aegis Proxy is running%s\n", colorGreen, colorReset)
	fmt.Printf("%s  API Key: %s%s\n", colorWhite, apiKey, colorReset)
	fmt.Printf("%s  Dashboard: http://%s%s\n", colorWhite, cfg.DashboardAddr(), colorReset)
	if cfg.DashboardPassword != "" {
		fmt.Printf("%s  Dashboard Password: %s%s\n", colorWhite, cfg.DashboardPassword, colorReset)
	}
	fmt.Printf("\n%sPress Ctrl+C to stop%s\n\n", colorYellow, colorReset)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		fmt.Printf("%sServer error: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	case <-sigChan:
		fmt.Printf("\n%s→ Shutting down gracefully...%s\n", colorYellow, colorReset)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := proxyServer.Shutdown(ctx); err != nil {
			fmt.Printf("%sError shutting down proxy server: %v%s\n", colorRed, err, colorReset)
		}

		if err := dashboardServer.Shutdown(ctx); err != nil {
			fmt.Printf("%sError shutting down dashboard server: %v%s\n", colorRed, err, colorReset)
		}

		fmt.Printf("%s✓ Shutdown complete%s\n", colorGreen, colorReset)
	}
}

func startWithHost(host string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sError loading config: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	cfg.ProxyHost = host
	cfg.DashboardHost = host

	if err := cfg.Save(); err != nil {
		fmt.Printf("%sError saving config: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	fmt.Printf("%s✓ Configured to bind to %s%s\n", colorGreen, host, colorReset)
	startServers()
}

func stopServer() {
	fmt.Printf("%s→ Stopping Aegis Proxy...%s\n", colorYellow, colorReset)
	fmt.Printf("%sNote: Use Ctrl+C to stop the running server%s\n", colorWhite, colorReset)
}

func showStatus() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sError loading config: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	fmt.Printf("%s=== Aegis Proxy Status ===%s\n\n", colorCyan, colorReset)
	fmt.Printf("%sProxy Address:%s %s\n", colorWhite, colorReset, cfg.ProxyAddr())
	fmt.Printf("%sDashboard Address:%s %s\n", colorWhite, colorReset, cfg.DashboardAddr())
	fmt.Printf("%sExpose to Network:%s %v\n", colorWhite, colorReset, cfg.ExposeToNetwork)
	fmt.Printf("%sUpstream Proxy:%s %s\n", colorWhite, colorReset, cfg.UpstreamProxy)
}

func handleAccountsCommand(args []string) {
	if len(args) == 0 {
		fmt.Printf("%sUsage: aegis accounts <list|add|remove>%s\n", colorYellow, colorReset)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sError loading config: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	db, err := database.InitDB(cfg.DBPath)
	if err != nil {
		fmt.Printf("%sError initializing database: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	defer db.Close()

	am := accounts.NewAccountManager(db)

	switch args[0] {
	case "list":
		listAccounts(am)
	case "add":
		if len(args) < 2 {
			fmt.Printf("%sUsage: aegis accounts add <file>%s\n", colorYellow, colorReset)
			return
		}
		addAccountsFromFile(am, args[1])
	case "remove":
		if len(args) < 2 {
			fmt.Printf("%sUsage: aegis accounts remove <email>%s\n", colorYellow, colorReset)
			return
		}
		removeAccount(am, args[1])
	default:
		fmt.Printf("%sUnknown accounts command: %s%s\n", colorRed, args[0], colorReset)
	}
}

func listAccounts(am *accounts.AccountManager) {
	accs, err := am.GetAll("")
	if err != nil {
		fmt.Printf("%sError listing accounts: %v%s\n", colorRed, err, colorReset)
		return
	}

	if len(accs) == 0 {
		fmt.Printf("%sNo accounts found%s\n", colorYellow, colorReset)
		return
	}

	fmt.Printf("%s=== Accounts ===%s\n\n", colorCyan, colorReset)
	for _, acc := range accs {
		statusColor := colorGreen
		if acc.Status != models.StatusActive {
			statusColor = colorRed
		}
		fmt.Printf("%s[%s]%s %s (%s)\n", statusColor, acc.Status, colorReset, acc.Email, acc.Provider)
	}
}

func addAccountsFromFile(am *accounts.AccountManager, filename string) {
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("%sError reading file: %v%s\n", colorRed, err, colorReset)
		return
	}

	lines := strings.Split(string(content), "\n")

	count := 0
	failed := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		email := strings.TrimSpace(parts[0])
		password := strings.TrimSpace(parts[1])

		fmt.Printf("%s→ Logging in %s...%s\n", colorCyan, email, colorReset)

		// Call Python runner to perform login
		result, err := auth.RunLogin(email, password)
		if err != nil {
			fmt.Printf("%sError logging in %s: %v%s\n", colorRed, email, err, colorReset)
			failed++
			continue
		}

		// Process each provider's credentials
		processProviderCredentials(am, email, password, "kiro", result.Kiro, &count, &failed)
		processProviderCredentials(am, email, password, "codebuddy", result.CodeBuddy, &count, &failed)
		processProviderCredentials(am, email, password, "wavespeed", result.Wavespeed, &count, &failed)
		processProviderCredentials(am, email, password, "canva", result.Canva, &count, &failed)
		processProviderCredentials(am, email, password, "yepapi", result.YepAPI, &count, &failed)
	}

	fmt.Printf("\n%s✓ Successfully added %d accounts%s\n", colorGreen, count, colorReset)
	if failed > 0 {
		fmt.Printf("%s✗ Failed to add %d accounts%s\n", colorRed, failed, colorReset)
	}
}

// processProviderCredentials handles adding an account and warming it up for a specific provider
func processProviderCredentials(am *accounts.AccountManager, email, password, providerName string, creds *auth.ProviderCredentials, count, failed *int) {
	if creds == nil || !creds.Success {
		if creds != nil && creds.Error != "" {
			log.Printf("provider=%s email=%s login_failed error=%s", providerName, email, creds.Error)
			// Track failed account
			trackFailedAccount(email, password, providerName)
		}
		return
	}

	// Serialize full credentials as JSON for account.Token
	// Providers expect Token to be JSON: {"access_token":"...","refresh_token":"...",...}
	tokenJSON := ""
	cookie := ""
	if creds.Credentials != nil {
		if credBytes, err := json.Marshal(creds.Credentials); err == nil {
			tokenJSON = string(credBytes)
		}
		for _, key := range []string{"cookie", "cookies", "session", "all_cookies"} {
			if c, ok := creds.Credentials[key].(string); ok && c != "" {
				cookie = c
				break
			}
		}
	}

	// Add account to database
	acc, err := am.Add(email, password, providerName)
	if err != nil {
		fmt.Printf("%sError adding %s to database (%s): %v%s\n", colorRed, email, providerName, err, colorReset)
		*failed++
		return
	}

	// Update token in DB and in-memory object
	if err := am.UpdateToken(acc.ID, tokenJSON, cookie); err != nil {
		fmt.Printf("%sWarning: Failed to update token for %s (%s): %v%s\n", colorYellow, email, providerName, err, colorReset)
	}
	acc.Token = tokenJSON
	acc.Cookie = cookie

	// Set status to active initially
	if err := am.UpdateStatus(acc.ID, models.StatusActive, ""); err != nil {
		fmt.Printf("%sWarning: Failed to update status for %s (%s): %v%s\n", colorYellow, email, providerName, err, colorReset)
	}

	// Get provider instance from registry
	prov, ok := provider.ProviderRegistry[providerName]
	if !ok {
		fmt.Printf("%sWarning: Provider %s not found in registry, skipping warmup%s\n", colorYellow, providerName, colorReset)
		fmt.Printf("%s✓ Added %s (%s)%s\n", colorGreen, email, providerName, colorReset)
		*count++
		return
	}

	// Warmup the account
	fmt.Printf("%s→ Warming up %s (%s)...%s\n", colorCyan, email, providerName, colorReset)
	if err := accounts.WarmupAccount(prov, acc, am.DB()); err != nil {
		log.Printf("warmup_failed email=%s provider=%s error=%v", email, providerName, err)
		// Update status to error on warmup failure
		if updateErr := am.UpdateStatus(acc.ID, models.StatusError, fmt.Sprintf("Warmup failed: %v", err)); updateErr != nil {
			fmt.Printf("%sWarning: Failed to update status after warmup failure for %s (%s): %v%s\n", colorYellow, email, providerName, updateErr, colorReset)
		}
		fmt.Printf("%s✓ Added %s (%s) - warmup failed, status set to error%s\n", colorGreen, email, providerName, colorReset)
	} else {
		log.Printf("warmup_success email=%s provider=%s", email, providerName)
		fmt.Printf("%s✓ Added %s (%s) - warmup successful%s\n", colorGreen, email, providerName, colorReset)
	}

	*count++
}

// trackFailedAccount appends failed account to ~/.aegis-proxy/failed-accounts.txt
func trackFailedAccount(email, password, provider string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	failedAccountsPath := filepath.Join(homeDir, ".aegis-proxy", "failed-accounts.txt")

	// Read existing entries to check for duplicates
	existingEntries := make(map[string]bool)
	if content, err := os.ReadFile(failedAccountsPath); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				existingEntries[line] = true
			}
		}
	}

	// Create entry
	entry := fmt.Sprintf("%s:%s:%s", email, password, provider)

	// Check if already exists
	if existingEntries[entry] {
		return
	}

	// Append to file
	f, err := os.OpenFile(failedAccountsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("failed to open failed-accounts.txt: %v", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(entry + "\n"); err != nil {
		log.Printf("failed to write to failed-accounts.txt: %v", err)
	}
}

func removeAccount(am *accounts.AccountManager, email string) {
	accs, err := am.GetAll("")
	if err != nil {
		fmt.Printf("%sError listing accounts: %v%s\n", colorRed, err, colorReset)
		return
	}

	for _, acc := range accs {
		if acc.Email == email {
			if err := am.Remove(acc.ID); err != nil {
				fmt.Printf("%sError removing account: %v%s\n", colorRed, err, colorReset)
				return
			}
			fmt.Printf("%s✓ Removed account: %s%s\n", colorGreen, email, colorReset)
			return
		}
	}

	fmt.Printf("%sAccount not found: %s%s\n", colorYellow, email, colorReset)
}

func handleAPIKeyCommand(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sError loading config: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	db, err := database.InitDB(cfg.DBPath)
	if err != nil {
		fmt.Printf("%sError initializing database: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	defer db.Close()

	if len(args) == 0 {
		key, err := auth.GetAPIKey(db)
		if err != nil {
			fmt.Printf("%sError getting API key: %v%s\n", colorRed, err, colorReset)
			return
		}
		fmt.Printf("%sAPI Key:%s %s\n", colorWhite, colorReset, key)
		return
	}

	if args[0] == "regen" {
		key, err := auth.RegenerateAPIKey(db)
		if err != nil {
			fmt.Printf("%sError regenerating API key: %v%s\n", colorRed, err, colorReset)
			return
		}
		fmt.Printf("%s✓ New API Key:%s %s\n", colorGreen, colorReset, key)
	}
}

func listModels() {
	allModels := models.ListAllModels()

	grouped := make(map[string][]models.ModelInfo)
	for _, model := range allModels {
		grouped[model.Tier] = append(grouped[model.Tier], model)
	}

	fmt.Printf("%s=== Available Models ===%s\n\n", colorCyan, colorReset)

	for tier, modelList := range grouped {
		fmt.Printf("%s[%s]%s\n", colorPurple, strings.ToUpper(tier), colorReset)
		for _, model := range modelList {
			fmt.Printf("  %s%s%s (%s)\n", colorWhite, model.Name, colorReset, model.Provider)
		}
		fmt.Println()
	}
}

func handleExposeCommand(args []string) {
	if len(args) == 0 {
		fmt.Printf("%sUsage: aegis expose <start|stop|status>%s\n", colorYellow, colorReset)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sError loading config: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	switch args[0] {
	case "start":
		cfg.ProxyHost = "0.0.0.0"
		cfg.DashboardHost = "0.0.0.0"
		cfg.ExposeToNetwork = true
		if err := cfg.Save(); err != nil {
			fmt.Printf("%sError saving config: %v%s\n", colorRed, err, colorReset)
			return
		}
		fmt.Printf("%s✓ Exposed to network (0.0.0.0)%s\n", colorGreen, colorReset)
		fmt.Printf("%sRestart the server for changes to take effect%s\n", colorYellow, colorReset)
	case "stop":
		cfg.ProxyHost = "127.0.0.1"
		cfg.DashboardHost = "127.0.0.1"
		cfg.ExposeToNetwork = false
		if err := cfg.Save(); err != nil {
			fmt.Printf("%sError saving config: %v%s\n", colorRed, err, colorReset)
			return
		}
		fmt.Printf("%s✓ Bound to localhost (127.0.0.1)%s\n", colorGreen, colorReset)
		fmt.Printf("%sRestart the server for changes to take effect%s\n", colorYellow, colorReset)
	case "status":
		if cfg.ExposeToNetwork {
			fmt.Printf("%sExposed to network: %sYES%s\n", colorWhite, colorGreen, colorReset)
		} else {
			fmt.Printf("%sExposed to network: %sNO%s\n", colorWhite, colorYellow, colorReset)
		}
	}
}

func setupPythonAuth() {
	fmt.Printf("%s=== Python Auth Setup ===%s\n\n", colorCyan, colorReset)

	// Find auth directory — check multiple locations
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	exeDir := filepath.Dir(exePath)
	homeDir, _ := os.UserHomeDir()

	authDirs := []string{
		filepath.Join(".", "auth"),                              // Current working directory
		filepath.Join(exeDir, "auth"),                           // Next to binary
		filepath.Join(exeDir, "..", "auth"),                     // Parent of binary (e.g. bin/../auth)
		filepath.Join(homeDir, ".aegis-proxy", "auth"),          // ~/.aegis-proxy/auth/
	}

	var authDir string
	for _, dir := range authDirs {
		if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
			authDir = dir
			break
		}
	}

	if authDir == "" {
		fmt.Printf("%s✗ Auth directory not found%s\n", colorRed, colorReset)
		fmt.Printf("%s  Expected 'auth/' folder with requirements.txt next to aegis binary%s\n", colorWhite, colorReset)
		return
	}

	fmt.Printf("%s→ Auth directory: %s%s\n", colorCyan, authDir, colorReset)

	// Detect Python 3.10+
	pythonCmd := ""
	for _, cmd := range []string{"python", "python3", "py"} {
		out, err := exec.Command(cmd, "-c", "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')").Output()
		if err == nil {
			version := strings.TrimSpace(string(out))
			parts := strings.Split(version, ".")
			if len(parts) >= 2 {
				major := 0
				minor := 0
				fmt.Sscanf(parts[0], "%d", &major)
				fmt.Sscanf(parts[1], "%d", &minor)
				if major >= 3 && minor >= 10 {
					pythonCmd = cmd
					fmt.Printf("%s→ Using %s (%s)%s\n", colorCyan, cmd, version, colorReset)
					break
				}
			}
		}
	}

	if pythonCmd == "" {
		fmt.Printf("%s✗ Python 3.10+ is required but not found%s\n", colorRed, colorReset)
		fmt.Printf("%s  Install Python 3.10+ from https://python.org and try again%s\n", colorWhite, colorReset)
		return
	}

	venvDir := filepath.Join(authDir, ".venv")

	// Create venv if it doesn't exist
	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		fmt.Printf("%s→ Creating virtual environment...%s\n", colorCyan, colorReset)
		cmd := exec.Command(pythonCmd, "-m", "venv", venvDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("%s✗ Failed to create venv: %v%s\n", colorRed, err, colorReset)
			return
		}
	} else {
		fmt.Printf("%s→ Virtual environment already exists%s\n", colorCyan, colorReset)
	}

	// Determine pip and python paths inside venv
	var pipPath, venvPython string
	if runtime.GOOS == "windows" {
		pipPath = filepath.Join(venvDir, "Scripts", "pip.exe")
		venvPython = filepath.Join(venvDir, "Scripts", "python.exe")
	} else {
		pipPath = filepath.Join(venvDir, "bin", "pip")
		venvPython = filepath.Join(venvDir, "bin", "python")
	}

	// Upgrade pip
	fmt.Printf("%s→ Upgrading pip...%s\n", colorCyan, colorReset)
	cmd := exec.Command(pipPath, "install", "--quiet", "--upgrade", "pip")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s! pip upgrade failed (non-critical): %v%s\n", colorYellow, err, colorReset)
	}

	// Install dependencies
	fmt.Printf("%s→ Installing dependencies...%s\n", colorCyan, colorReset)
	reqFile := filepath.Join(authDir, "requirements.txt")
	cmd = exec.Command(pipPath, "install", "--quiet", "-r", reqFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s✗ Failed to install dependencies: %v%s\n", colorRed, err, colorReset)
		return
	}

	// Install Camoufox browser
	fmt.Printf("%s→ Fetching Camoufox browser (this may take a minute)...%s\n", colorCyan, colorReset)
	cmd = exec.Command(venvPython, "-m", "camoufox", "fetch")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s! Camoufox fetch failed: %v%s\n", colorYellow, err, colorReset)
		fmt.Printf("%s  You can try manually: %s -m camoufox fetch%s\n", colorWhite, venvPython, colorReset)
	}

	fmt.Printf("\n%s✓ Auth automation setup complete%s\n", colorGreen, colorReset)
	fmt.Printf("%s✓ Python venv: %s%s\n", colorGreen, venvDir, colorReset)
}

func handleMITMCommand(args []string) {
	if len(args) == 0 {
		fmt.Printf("%sUsage: aegis mitm <enable|disable|status|setup-ca|setup-hosts|setup-trust|start|stop>%s\n", colorYellow, colorReset)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%sError loading config: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	mitmProxy := mitm.NewMITMProxy(cfg)

	switch args[0] {
	case "enable":
		fmt.Printf("%s=== Enabling MITM Proxy ===%s\n\n", colorCyan, colorReset)
		
		fmt.Printf("%s[1/3] Setting up CA certificate...%s\n", colorWhite, colorReset)
		if err := mitmProxy.SetupCA(); err != nil {
			fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ CA certificate ready%s\n\n", colorGreen, colorReset)

		fmt.Printf("%s[2/3] Setting up hosts file...%s\n", colorWhite, colorReset)
		if err := mitmProxy.SetupHosts(); err != nil {
			fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
			fmt.Printf("%sNote: You may need to run this command with administrator/root privileges%s\n", colorYellow, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Hosts file configured%s\n\n", colorGreen, colorReset)

		fmt.Printf("%s[3/3] Installing CA to trust store...%s\n", colorWhite, colorReset)
		if err := mitmProxy.SetupTrustStore(); err != nil {
			fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
			fmt.Printf("%sNote: You may need to run this command with administrator/root privileges%s\n", colorYellow, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ CA certificate installed%s\n\n", colorGreen, colorReset)

		fmt.Printf("%s=== MITM Proxy Enabled ===%s\n\n", colorGreen, colorReset)
		fmt.Printf("%sNext steps:%s\n", colorWhite, colorReset)
		fmt.Printf("1. Start MITM proxy: %saegis mitm start%s\n", colorCyan, colorReset)
		fmt.Printf("2. Configure your tools to use proxy: %shttp://127.0.0.1:8443%s\n", colorCyan, colorReset)

	case "disable":
		fmt.Printf("%s=== Disabling MITM Proxy ===%s\n\n", colorCyan, colorReset)
		
		if err := mitmProxy.Cleanup(); err != nil {
			fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
			fmt.Printf("%sNote: You may need to run this command with administrator/root privileges%s\n", colorYellow, colorReset)
			os.Exit(1)
		}
		
		fmt.Printf("%s✓ MITM proxy disabled%s\n", colorGreen, colorReset)
		fmt.Printf("%s✓ Hosts file cleaned up%s\n", colorGreen, colorReset)
		fmt.Printf("%s✓ CA certificate uninstalled%s\n", colorGreen, colorReset)

	case "status":
		fmt.Printf("%s=== MITM Proxy Status ===%s\n\n", colorCyan, colorReset)
		
		certManager := mitm.NewCertificateManager(cfg.DataDir)
		if certManager.CertExists() {
			fmt.Printf("%sCA Certificate: %sInstalled%s\n", colorWhite, colorGreen, colorReset)
		} else {
			fmt.Printf("%sCA Certificate: %sNot installed%s\n", colorWhite, colorYellow, colorReset)
		}

		hostsManager := mitm.NewHostsManager()
		entries, _ := hostsManager.GetEntries()
		if len(entries) > 0 {
			fmt.Printf("%sHosts Entries: %s%d configured%s\n", colorWhite, colorGreen, len(entries), colorReset)
			for _, entry := range entries {
				fmt.Printf("  - %s -> %s\n", entry.Domain, entry.IP)
			}
		} else {
			fmt.Printf("%sHosts Entries: %sNone%s\n", colorWhite, colorYellow, colorReset)
		}

		trustManager := mitm.NewTrustStoreManager(certManager.GetCertPath())
		if trustManager.IsCertInstalled() {
			fmt.Printf("%sTrust Store: %sInstalled%s\n", colorWhite, colorGreen, colorReset)
		} else {
			fmt.Printf("%sTrust Store: %sNot installed%s\n", colorWhite, colorYellow, colorReset)
		}

	case "setup-ca":
		if err := mitmProxy.SetupCA(); err != nil {
			fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ CA certificate generated%s\n", colorGreen, colorReset)

	case "setup-hosts":
		if err := mitmProxy.SetupHosts(); err != nil {
			fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ Hosts file configured%s\n", colorGreen, colorReset)

	case "setup-trust":
		if err := mitmProxy.SetupTrustStore(); err != nil {
			fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ CA certificate installed to trust store%s\n", colorGreen, colorReset)

	case "start":
		fmt.Printf("%s=== Starting MITM Proxy ===%s\n\n", colorCyan, colorReset)
		fmt.Printf("%sListening on port 8443...%s\n", colorWhite, colorReset)
		
		if err := mitmProxy.Start(); err != nil {
			fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}

	case "stop":
		fmt.Printf("%s=== Stopping MITM Proxy ===%s\n\n", colorCyan, colorReset)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := mitmProxy.Shutdown(ctx); err != nil {
			fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s✓ MITM proxy stopped%s\n", colorGreen, colorReset)

	default:
		fmt.Printf("%sUnknown mitm command: %s%s\n", colorRed, args[0], colorReset)
		fmt.Printf("%sUsage: aegis mitm <enable|disable|status|setup-ca|setup-hosts|setup-trust|start|stop>%s\n", colorYellow, colorReset)
	}
}

func showVersion() {
	fmt.Printf("%sAegis Proxy v1.0.0%s\n", colorCyan, colorReset)
}

func showHelp() {
	fmt.Printf("%s=== Aegis Proxy CLI ===%s\n\n", colorCyan, colorReset)
	fmt.Printf("%sUsage:%s aegis <command> [options]\n\n", colorWhite, colorReset)
	fmt.Printf("%sCommands:%s\n", colorWhite, colorReset)
	fmt.Printf("  %sstart%s              Start proxy and dashboard servers\n", colorGreen, colorReset)
	fmt.Printf("  %sstart --host <ip>%s  Start and bind to specific IP\n", colorGreen, colorReset)
	fmt.Printf("  %sstop%s               Stop the proxy\n", colorGreen, colorReset)
	fmt.Printf("  %sstatus%s             Show service status\n", colorGreen, colorReset)
	fmt.Printf("  %saccounts list%s      List all accounts\n", colorGreen, colorReset)
	fmt.Printf("  %saccounts add <file>%s Batch add accounts from file\n", colorGreen, colorReset)
	fmt.Printf("  %saccounts remove <email>%s Remove account\n", colorGreen, colorReset)
	fmt.Printf("  %sapikey%s             Show API key\n", colorGreen, colorReset)
	fmt.Printf("  %sapikey regen%s       Regenerate API key\n", colorGreen, colorReset)
	fmt.Printf("  %smodels%s             List available models\n", colorGreen, colorReset)
	fmt.Printf("  %sexpose start%s       Expose to network (0.0.0.0)\n", colorGreen, colorReset)
	fmt.Printf("  %sexpose stop%s        Bind back to localhost\n", colorGreen, colorReset)
	fmt.Printf("  %sexpose status%s      Show expose status\n", colorGreen, colorReset)
	fmt.Printf("  %smitm enable%s        Enable MITM proxy\n", colorGreen, colorReset)
	fmt.Printf("  %smitm disable%s       Disable MITM proxy\n", colorGreen, colorReset)
	fmt.Printf("  %smitm status%s        Show MITM proxy status\n", colorGreen, colorReset)
	fmt.Printf("  %smitm start%s         Start MITM proxy server\n", colorGreen, colorReset)
	fmt.Printf("  %smitm stop%s          Stop MITM proxy server\n", colorGreen, colorReset)
	fmt.Printf("  %ssetup%s              Set up Python auth automation\n", colorGreen, colorReset)
	fmt.Printf("  %sversion%s            Show version\n", colorGreen, colorReset)
	fmt.Printf("  %shelp%s               Show this help\n", colorGreen, colorReset)
}

func printBanner() {
	banner := `
   ___   ______ _____ _____  _____ 
  / _ \ |  ____/ ____|_   _|/ ____|
 | |_| || |__ | |  __  | | | (___  
 |  _  ||  __|| | |_ | | |  \___ \ 
 | | | || |___| |__| |_| |_ ____) |
 |_| |_||______\_____|_____|_____/ 
                                   
`
	fmt.Printf("%s%s%s", colorPurple, banner, colorReset)
	fmt.Printf("%sAegis Proxy - AI Model Gateway%s\n\n", colorCyan, colorReset)
}
