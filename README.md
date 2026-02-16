# Openkh

A Telegram bot that connects to [OpenCode](https://opencode.ai) AI via its REST API, providing real-time streaming responses, session management, and full conversation persistence.

## Live Demo

The bot powers the production website: **[openkh.khaledxab.com](https://openkh.khaledxab.com)**

## Architecture

```
Telegram User <-> Bot (Go) <-> OpenCode Server (REST API + SSE)
                                     |
                               localhost:4096
```

The bot communicates with OpenCode through its HTTP API. Responses stream in real-time via Server-Sent Events (SSE), editing the Telegram message as tokens arrive.

### Project Structure

```
github.com/Khaledxab/Openkh/
├── cmd/openkh/main.go              # Entry point, dependency wiring
├── internal/
│   ├── config/config.go            # Env-based config, portable DB path resolution
│   ├── store/store.go              # SQLite session storage (chat -> session mapping)
│   ├── opencode/
│   │   ├── types.go                # API types + SSE event types
│   │   ├── client.go               # OpenCode HTTP client
│   │   └── stream.go               # SSE StreamManager + MessageSender interface
│   └── telegram/
│       ├── bot.go                  # Bot struct, handler registration, TelegramSender adapter
│       ├── commands.go             # /start /help /new /stop /clear /model /think
│       ├── sessions.go             # /sessions /switch /rename /delete /purge /diff /history
│       ├── agents.go               # /agent command + dynamic agent config
│       ├── callbacks.go            # Default message handler + callback query routing
│       ├── info.go                 # /status /stats
│       ├── middleware.go           # Auth allowlist, rate limiting, admin check
│       └── helpers.go              # shortID, currentSessionID, currentAgent
├── Makefile
├── Dockerfile
├── .env.example
└── start.sh
```

## Features

### Core
- **Streaming responses** — messages update in real-time as the AI generates text
- **Thinking indicator** — shows status while the AI reasons, then displays only the final response
- **Session persistence** — conversations preserved across messages using OpenCode sessions
- **Dynamic agents** — switch between AI agents (e.g. `sisyphus` for coding, `oracle` for deep analysis)
- **Async prompts** — non-blocking `promptAsync` API, results arrive via SSE events

### Commands

| Command | Description |
|---------|-------------|
| `/start` | Welcome screen with reply keyboard |
| `/help` | List all available commands |
| `/new` | Start a fresh conversation |
| `/stop` | Abort the current AI operation |
| `/sessions` | List all sessions with inline switch buttons |
| `/switch <id>` | Switch to a specific session |
| `/rename <title>` | Rename the current session |
| `/delete [id]` | Delete current or specified session |
| `/purge` | Delete all sessions (admin only) |
| `/agent` | Switch agent via inline keyboard |
| `/agent <name>` | Set agent directly |
| `/diff` | Show file changes in current session |
| `/history` | Show last 10 messages |
| `/status` | Bot uptime, active streams, current session/agent |
| `/stats` | Total messages and session count |
| `/clear` | Delete current session from bot DB and OpenCode |
| `/model` | Show current model |
| `/think` | Toggle thinking display |

### Security
- **User allowlist** — only authorized Telegram user IDs can interact
- **Admin users** — certain commands (e.g. `/purge`) restricted to admins
- **Rate limiting** — 2-second cooldown between messages per user

## Requirements

- Go 1.21+
- CGO enabled (for SQLite via `mattn/go-sqlite3`)
- OpenCode server running (`opencode serve --port 4096`)
- Telegram Bot Token (from [@BotFather](https://t.me/BotFather))

## Setup

### 1. Create a Telegram Bot

1. Talk to [@BotFather](https://t.me/BotFather) on Telegram
2. Send `/newbot` and follow the prompts
3. Copy the bot token

### 2. Start OpenCode Server

```bash
opencode serve --hostname 0.0.0.0 --port 4096
```

### 3. Configure Environment

Copy `.env.example` and fill in your values:

```bash
cp .env.example .env
```

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TELEGRAM_BOT_TOKEN` | Yes | — | Telegram bot token from BotFather |
| `OPENCODE_URL` | No | `http://localhost:4096` | OpenCode server URL |
| `ALLOWED_USERS` | No | — (allow all) | Comma-separated Telegram user IDs |
| `ADMIN_USERS` | No | — (all are admin) | Comma-separated admin user IDs |
| `WORK_DIR` | No | `.` | Working directory |
| `DB_PATH` | No | `~/.local/share/openkh/openkh.db` | Database file path |
| `DATA_DIR` | No | — | Data directory (DB at `$DATA_DIR/openkh.db`) |
| `AGENTS` | No | `sisyphus,oracle` | Agent config: `name:desc,name:desc` |

To find your Telegram user ID, send a message to [@userinfobot](https://t.me/userinfobot).

**Note:** `.env` uses plain `KEY=VALUE` format (no `export` prefix) for systemd `EnvironmentFile` compatibility.

### 4. Build & Run

```bash
make build
make run
```

Or manually:

```bash
CGO_ENABLED=1 go build -o bin/openkh ./cmd/openkh
./bin/openkh
```

### 5. Deploy with start.sh

```bash
bash start.sh
```

This kills any existing bot process, sources `.env`, and starts the bot in the background.

## Docker

```bash
docker build -t openkh .
docker run --env-file .env openkh
```

## Systemd Service (Production)

Create `/etc/systemd/system/openkh.service`:

```ini
[Unit]
Description=Openkh Telegram Bot
After=network.target

[Service]
Type=simple
User=your_user
WorkingDirectory=/path/to/opencode-bot-go
EnvironmentFile=/path/to/opencode-bot-go/.env
ExecStart=/path/to/opencode-bot-go/bin/openkh
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable openkh
sudo systemctl start openkh
journalctl -u openkh -f
```

## How It Works

### Message Flow

1. User sends a text message in Telegram
2. Bot checks auth and rate limit
3. Bot looks up or creates an OpenCode session for this user
4. Bot sends a placeholder "Thinking..." message
5. Bot registers the session with the SSE StreamManager
6. Bot calls `POST /session/:id/prompt_async` (returns immediately)
7. SSE events arrive and edit the message in real-time
8. On completion, final text is set and state is cleaned up

### SSE Event Handling

The bot maintains a persistent connection to `GET /event` on the OpenCode server. Events are filtered by session ID and routed to the correct Telegram chat. Message edits are throttled to 1/second to respect Telegram API limits.

### Agent System

Each chat stores its preferred agent in the database. The agent name is passed in every `PromptAsync` call. Default agents are `sisyphus` (General coding) and `oracle` (Deep analysis). Configure custom agents via the `AGENTS` environment variable.

## OpenCode API Endpoints Used

| Method | Endpoint | Purpose |
|--------|----------|---------|
| `GET` | `/global/health` | Server health check on startup |
| `POST` | `/session` | Create new session |
| `GET` | `/session` | List all sessions |
| `GET` | `/session/:id` | Get session details |
| `PATCH` | `/session/:id` | Rename session |
| `DELETE` | `/session/:id` | Delete session |
| `GET` | `/session/:id/message` | Get message history |
| `POST` | `/session/:id/prompt_async` | Send async prompt |
| `POST` | `/session/:id/abort` | Cancel running operation |
| `GET` | `/session/:id/diff` | Get file changes |
| `GET` | `/event` | SSE event stream |

## Dependencies

- [go-telegram/bot](https://github.com/go-telegram/bot) v1.18.0 — Telegram Bot API
- [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) v1.14.34 — SQLite driver (requires CGO)

## License

MIT
