# Aegis Proxy

Self-hosted AI proxy that runs locally on your machine. Access premium AI models through OpenAI and Anthropic compatible API endpoints.

## Features

- **OpenAI + Anthropic Compatible** — Drop-in replacement at `localhost:3130`. Works with Cursor, VS Code, Continue, Cline, and any OpenAI/Anthropic compatible tool.
- **20+ AI Models** — Claude, Gemini, GPT, DeepSeek, GLM, Kimi and more through a unified endpoint.
- **AI Image Generation** — Generate images via `POST /v1/images/generations` using Canva AI. OpenAI DALL-E compatible endpoint.
- **Three Tiers** — Standard models (Claude family via Kiro), MAX models (GPT-5, Gemini, Opus 4.6, DeepSeek, Kimi via CodeBuddy), and Canva (image generation).
- **Web Dashboard** — Manage accounts, view logs, monitor credits, and configure settings at `localhost:3131`.
- **Built-in Chat UI** — Chat interface at `localhost:3130/chat` with streaming support.
- **Smart Routing** — Sticky account rotation, automatic error recovery, multi-step fallback degradation.
- **Proxy Pool** — Route upstream requests through HTTP/SOCKS5 proxies with per-provider routing, latency-based selection, and auto-testing.
- **Content Filters** — 31 obfuscation rules with template system (basic, aggressive, minimal). Prevents upstream content filtering from blocking requests.
- **Account Warmup** — Validates new accounts immediately after login with a test request.
- **JSONL Request Logging** — Structured request logs with model, provider, latency, status, and token counts. Auto-rotation at 100MB.
- **Bcrypt Security** — Dashboard password hashed with bcrypt. Auto-migrates plaintext passwords on startup.
- **LAN Access** — Bind to a specific IP with `--host` flag or expose to all interfaces with `expose start`.
- **IP Whitelist** — Restrict access to specific IP addresses when exposed to the network.
- **MITM Proxy** — Built-in HTTPS man-in-the-middle proxy for Cursor, Trae, and Windsurf.
- **Cross-Platform** — Linux, macOS, and Windows.

## Supported Models

### Standard Tier (Kiro)

| Model | ID |
|-------|-----|
| Auto (best available) | `auto` |
| Claude Sonnet 4.5 | `claude-sonnet-4.5` |
| Claude Sonnet 4 | `claude-sonnet-4` |
| Claude Haiku 4.5 | `claude-haiku-4.5` |
| DeepSeek 3.2 | `deepseek-3.2` |
| MiniMax M2.5 | `minimax-m2.5` |
| GLM-5 | `glm-5` |
| Qwen3 Coder Next | `qwen3-coder-next` |

### MAX Tier (CodeBuddy)

| Model | ID |
|-------|-----|
| Claude Opus 4.6 | `claude-opus-4.6` |
| Gemini 2.5 Pro | `gemini-2.5-pro` |
| Gemini 2.5 Flash | `gemini-2.5-flash` |
| GPT-5.4 | `gpt-5.4` |
| GPT-5.2 | `gpt-5.2` |
| DeepSeek V3-2 | `deepseek-v3-2` |
| Kimi K2.5 | `kimi-k2.5` |

### Wavespeed Tier

| Model | ID |
|-------|-----|
| Wavespeed Claude Sonnet 4.5 | `wavespeed-claude-sonnet-4.5` |
| Wavespeed Claude Sonnet 4 | `wavespeed-claude-sonnet-4` |
| Wavespeed GPT-5 | `wavespeed-gpt-5` |

### Windsurf Tier

| Model | ID |
|-------|-----|
| Windsurf Claude Sonnet 4 | `windsurf-claude-sonnet-4` |
| Windsurf GPT-5 | `windsurf-gpt-5` |

### Canva Tier (Image Generation)

| Model | ID |
|-------|-----|
| Canva Image | `canva-image` |

Use the OpenAI-compatible image generation endpoint: `POST /v1/images/generations` with `"model": "canva-image"`.

## Quick Start

```bash
# 1. Build the binary
go build -o aegis.exe ./cmd/aegis

# 2. Set up Python auth automation
aegis setup

# 3. Start the proxy
aegis start

# 4. Add accounts (via dashboard or CLI)
aegis accounts add accounts.txt
```

The proxy runs at `localhost:3130` and the dashboard at `localhost:3131`.

## Usage

Point any OpenAI or Anthropic compatible tool to your local proxy:

```
Base URL: http://localhost:3130/v1
API Key:  (run: aegis apikey)
```

**Cursor / VS Code:** Set `OPENAI_API_KEY` and `OPENAI_BASE_URL` in your editor settings, or configure the custom API endpoint to `http://localhost:3130/v1`.

**cURL — OpenAI format:**

```bash
curl http://localhost:3130/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $(aegis apikey 2>/dev/null)" \
  -d '{
    "model": "claude-sonnet-4.5",
    "messages": [
      {"role": "system", "content": "You are a helpful coding assistant."},
      {"role": "user", "content": "Hello!"}
    ],
    "stream": true
  }'
```

**cURL — Anthropic format:**

```bash
curl http://localhost:3130/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: $(aegis apikey 2>/dev/null)" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4",
    "max_tokens": 1024,
    "system": "You are a helpful coding assistant.",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**cURL — OpenAI Responses API:**

```bash
curl http://localhost:3130/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $(aegis apikey 2>/dev/null)" \
  -d '{
    "model": "claude-sonnet-4.5",
    "instructions": "You are a helpful coding assistant.",
    "input": "Hello!",
    "stream": true
  }'
```

**Image Generation (OpenAI DALL-E compatible):**

```bash
curl http://localhost:3130/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $(aegis apikey 2>/dev/null)" \
  -d '{
    "model": "canva-image",
    "prompt": "A futuristic cityscape at sunset with flying cars",
    "n": 4,
    "size": "1024x1024"
  }'
```

## Adding Accounts

**Important Notes:**
- Use fresh Google accounts. We strongly recommend using newly created, temporary Google accounts.
- No verification required. The Google account must NOT have any pending verification (phone verification, 2FA, security prompts). Any verification step will cause the automated login to fail.

### Via Dashboard

Open `http://localhost:3131` and navigate to Accounts. Click Add Accounts. Enter one account per line in `email:password` format.

### Via CLI

Create a text file with one account per line:

```
user1@gmail.com:password123
user2@gmail.com|mypassword456
user3@gmail.com:securepass789
```

Supported separators: `:` or `|`

Then run:

```bash
aegis accounts add accounts.txt
```

After adding, each account is automatically warmed up with a test request to verify credentials work before production use.

## Commands

```
aegis                              Start the proxy (foreground)
aegis start                        Start proxy and dashboard servers
aegis start --host <ip>            Start and bind to specific IP (LAN access)
aegis stop                         Stop the proxy
aegis status                       Show service status

aegis accounts list                List all accounts
aegis accounts add <file>          Batch add from file (email:password per line)
aegis accounts remove <email>      Remove account

aegis apikey                       Show API key
aegis apikey regen                 Regenerate API key
aegis models                       List available models

aegis expose start                 Expose proxy & dashboard to network (0.0.0.0)
aegis expose stop                  Bind back to localhost only (127.0.0.1)
aegis expose status                Show expose status

aegis mitm enable                  Setup MITM proxy (CA cert + hosts + trust store)
aegis mitm disable                 Remove MITM proxy and cleanup
aegis mitm status                  Show MITM proxy status
aegis mitm setup-ca                Generate CA certificate only
aegis mitm setup-hosts             Configure hosts file only
aegis mitm setup-trust             Install CA to trust store only
aegis mitm start                   Start MITM proxy server (port 8443)
aegis mitm stop                    Stop MITM proxy server

aegis setup                        Set up Python auth automation
aegis version                      Show version
aegis help                         Show help
```

## Dashboard

Access the web dashboard at `http://localhost:3131`:

- **Dashboard** — Overview with Standard/MAX account stats and credit usage
- **Accounts** — Add/remove accounts with batch import, provider status indicators
- **Models** — View all available models grouped by tier
- **Logs** — Monitor request history, latency, and errors
- **API Key** — View or regenerate your local API key
- **Proxy** — Manage HTTP/SOCKS5 proxies with auto-testing and provider routing
- **Settings** — General settings, network configuration, and MITM proxy management

## API Endpoints

### Proxy Server (Port 3130)

| Endpoint | Format | Description |
|----------|--------|-------------|
| `POST /v1/chat/completions` | OpenAI | Chat completions (streaming & non-streaming) |
| `POST /v1/responses` | OpenAI | Responses API (streaming & non-streaming) |
| `POST /v1/messages` | Anthropic | Messages API (streaming & non-streaming) |
| `POST /v1/images/generations` | OpenAI | Image generation (Canva AI) |
| `GET /v1/models` | OpenAI | List available models with tier info |
| `GET /chat` | — | Built-in chat UI |
| `GET /health` | — | Health check |

### Dashboard API (Port 3131)

| Endpoint | Description |
|----------|-------------|
| `POST /api/auth/login` | Dashboard login |
| `POST /api/auth/logout` | Dashboard logout |
| `GET /api/dashboard/stats` | Overview stats |
| `GET /api/accounts` | List accounts |
| `POST /api/accounts` | Add accounts |
| `DELETE /api/accounts` | Remove account |
| `POST /api/accounts/sync` | Sync accounts |
| `DELETE /api/accounts/inactive` | Delete inactive accounts |
| `DELETE /api/accounts/all` | Delete all accounts |
| `GET /api/models` | List models |
| `GET /api/logs` | Request logs |
| `GET /api/apikey` | Get API key |
| `POST /api/apikey/regen` | Regenerate API key |
| `GET /api/proxies` | List proxies |
| `POST /api/proxies` | Add proxy |
| `DELETE /api/proxies` | Remove proxy |
| `POST /api/proxies/test` | Test all proxies |
| `GET /api/settings` | Get settings |
| `PUT /api/settings` | Update settings |

### Request Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `model` | string | Model ID (see model list above) |
| `messages` | array | Conversation messages |
| `stream` | boolean | Enable SSE streaming (recommended) |
| `max_tokens` | integer | Maximum output tokens |
| `temperature` | float | Sampling temperature (0-2) |
| `tools` | array | Tool/function definitions for tool use |

## Content Filters

31 text replacement rules that automatically obfuscate sensitive terms in requests before sending to upstream providers. This prevents upstream content filtering from blocking your requests.

Three built-in templates:
- **basic** — 9 essential rules
- **aggressive** — 31 rules with full obfuscation (default)
- **minimal** — 3 rules for light filtering

Examples of obfuscation rules:
- `Claude` → `CL4ude`
- `OpenAI` → `0penAI`
- `Anthropic` → `Anxthxropic`
- `jailbreak` → `J41lbreak`
- `cache_control` → removed

Request content is obfuscated before sending, response content is de-obfuscated before returning to you. Manage filters via the dashboard or config file.

## Proxy Pool

Route upstream requests through HTTP/SOCKS5 proxies with advanced features:

- **Per-provider routing** — Assign proxies to specific providers (Kiro, CodeBuddy, Wavespeed, etc.)
- **Latency-based selection** — Automatically picks the lowest latency proxy
- **Auto-testing** — Background goroutine periodically tests all proxies
- **Health tracking** — Proxies marked as ok/failed based on connectivity tests
- **Region support** — Track proxy regions for geo-specific routing
- **Auto-delete** — Optionally remove failed proxies automatically

Proxy configuration stored at `~/.aegis-proxy/proxies.json`.

## Smart Routing

### Sticky Sessions

Clients are assigned a consistent account per provider. The mapping persists across restarts via `~/.aegis-proxy/sticky-sessions.json`. When a sticky account fails, it automatically rotates to the next available account.

### Auto-Recovery

Background goroutine runs every 5 minutes to recover errored accounts:
- 2-minute cooldown before retry attempts
- Validates account credentials
- Automatically reactivates recovered accounts
- Logs all recovery attempts

### Account Warmup

New accounts are validated immediately after login with a minimal test request (`max_tokens: 1`). Failed warmups set the account status to error, preventing it from being used in production routing.

## Network Access

### LAN Access

By default, aegis binds to `127.0.0.1` (localhost only). To access from other devices on your network:

```bash
# Bind to a specific IP
aegis start --host 192.168.1.100

# Expose to all interfaces (0.0.0.0)
aegis expose start

# Bind back to localhost
aegis expose stop
```

### MITM Proxy

Built-in HTTPS MITM proxy for tools that require direct API access (Cursor, Trae, Windsurf):

```bash
aegis mitm enable     # Setup MITM proxy
aegis mitm status     # Check status
aegis mitm disable    # Remove MITM proxy
```

## Project Structure

```
aegis-proxy/
├── cmd/aegis/
│   └── main.go                          # CLI entry point
├── internal/
│   ├── accounts/
│   │   ├── manager.go                   # Account management, sticky sessions, auto-recovery
│   │   └── warmup.go                    # Account warmup validation
│   ├── api/
│   │   ├── api.go                       # Dashboard REST API handlers
│   │   └── auth_middleware.go           # Dashboard auth (bcrypt)
│   ├── auth/
│   │   └── apikey.go                    # API key management
│   ├── config/
│   │   └── config.go                    # Configuration management
│   ├── dashboard/
│   │   ├── server.go                    # Dashboard server
│   │   └── embed.go                     # Static file embedding
│   ├── database/
│   │   ├── database.go                  # SQLite layer (modernc.org/sqlite)
│   │   └── migrations.go               # Schema migrations
│   ├── filter/
│   │   ├── filter.go                    # Filter engine with modes
│   │   ├── default_filters.go           # 31 obfuscation rules
│   │   └── templates.go                 # Filter templates (basic/aggressive/minimal)
│   ├── logger/
│   │   ├── logger.go                    # Request logging
│   │   └── jsonl_logger.go             # JSONL file logger with rotation
│   ├── middleware/
│   │   ├── auth.go                      # Auth middleware
│   │   ├── cors.go                      # CORS middleware
│   │   └── logging.go                   # Logging middleware
│   ├── mitm/                            # MITM proxy
│   ├── models/
│   │   ├── models.go                    # Data structures
│   │   └── model_registry.go            # Model mapping (20+ models)
│   ├── provider/
│   │   ├── provider.go                  # Provider interface
│   │   ├── kiro/                        # Kiro provider (Standard tier)
│   │   ├── codebuddy/                   # CodeBuddy provider (MAX tier)
│   │   ├── windsurf/                    # Windsurf provider
│   │   ├── canva/                       # Canva provider (Image generation)
│   │   ├── wavespeed/                   # Wavespeed provider
│   │   ├── yepapi/                      # YepAPI provider
│   │   └── codex/                       # Codex provider
│   ├── proxy/
│   │   ├── server.go                    # Proxy server
│   │   ├── handlers.go                  # Request handlers (real provider calls)
│   │   ├── openai.go                    # OpenAI format
│   │   ├── anthropic.go                 # Anthropic format
│   │   └── errors.go                    # Error handling
│   ├── proxypool/
│   │   ├── pool.go                      # Proxy pool manager
│   │   ├── config.go                    # Proxy pool config
│   │   └── tester.go                    # Proxy auto-tester
│   ├── router/
│   │   └── router.go                    # Smart routing
│   └── sync/                            # Account syncing
├── auth/                                # Python browser automation
│   ├── login.py                         # Auto-login script
│   ├── login_vip.py                     # VIP login flow
│   ├── canva_generate.py                # Canva image generation
│   ├── codex_login.py                   # Codex login
│   ├── requirements.txt                 # Python dependencies
│   ├── setup.sh                         # Setup script
│   └── app/
│       └── providers/                   # Provider-specific login flows
├── web/dashboard/                       # Svelte dashboard
├── go.mod
├── go.sum
└── aegis.exe                            # Compiled binary (~16 MB)
```

## Configuration

### Default Paths

- **Config:** `~/.aegis-proxy/config.json`
- **Database:** `~/.aegis-proxy/aegis.db`
- **Proxies:** `~/.aegis-proxy/proxies.json`
- **Sticky Sessions:** `~/.aegis-proxy/sticky-sessions.json`
- **Request Logs:** `~/.aegis-proxy/request_logs.jsonl`
- **Failed Accounts:** `~/.aegis-proxy/failed-accounts.txt`

### Ports

- **Proxy:** 3130
- **Dashboard:** 3131

### Environment Variables

- `AEGIS_PROXY_HOST` — Proxy server host (default: `127.0.0.1`)
- `AEGIS_DASHBOARD_HOST` — Dashboard host (default: `127.0.0.1`)

## Build

### Requirements

- Go 1.25.5+
- Python 3.10+ (for auto-login)

### Build from Source

```bash
git clone https://github.com/aegis-proxy/aegis.git
cd aegis
go build -o aegis.exe ./cmd/aegis
```

### Setup Python Auth

```bash
aegis setup
```

This sets up the Python virtual environment and installs Camoufox (stealth Firefox) for automated account login.

## Database Schema

### accounts
```sql
id, email, password, provider, status, credits_used, credits_total,
token, cookie, last_used_at, last_synced_at, error_message,
created_at, updated_at
```

### request_logs
```sql
id, model, provider, account_id, status, status_code,
prompt_tokens, completion_tokens, total_tokens, latency_ms,
error_message, ip_address, created_at
```

### settings
```sql
key, value, updated_at
```

## Troubleshooting

### Port already in use

```bash
aegis stop
aegis start
```

### Connection refused

```bash
# Check if the service is running
aegis status

# Start the service
aegis start
```

### All accounts exhausted

Add more accounts via the dashboard or CLI. Credits reset daily — wait for the next reset cycle.

### Auth automation not working

```bash
aegis setup    # Re-initialize the auth system
```

## Tech Stack

- **Backend:** Go 1.25.5
- **Database:** SQLite (modernc.org/sqlite — pure Go, no CGO)
- **Browser Automation:** Python + Camoufox (stealth Firefox)
- **Dashboard:** Svelte
- **Security:** bcrypt password hashing

## License

MIT
