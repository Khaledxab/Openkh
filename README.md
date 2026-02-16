# OpenCode Telegram Bot

A Telegram bot that connects to [OpenCode](https://opencode.ai) AI via its REST API, providing real-time streaming responses, session management, and full conversation persistence.

## Architecture

```
Telegram User <-> Bot (Go) <-> OpenCode Server (REST API + SSE)
                                     |
                               localhost:4096
```

The bot communicates with OpenCode through its HTTP API instead of spawning CLI processes. Responses stream in real-time via Server-Sent Events (SSE), editing the Telegram message as tokens arrive.

### Project Structure

```
opencode-bot-go/
├── main.go          # Entry point, wiring
├── config.go        # Environment-based configuration
├── opencode.go      # OpenCode REST API client
├── stream.go        # SSE event listener + Telegram message updater
├── handlers.go      # Telegram command handlers
├── sessions.go      # SQLite session storage
├── middleware.go     # Auth allowlist + rate limiting
├── .env             # Environment variables (not committed)
└── bot.db           # SQLite database (created on first run)
```

## Features

### Core
- **Streaming responses** — messages update in real-time as the AI generates text
- **Thinking indicator** — shows "Thinking..." while the AI reasons, then displays only the final response
- **Session persistence** — conversations are preserved across messages using OpenCode sessions
- **Async prompts** — non-blocking `promptAsync` API, results arrive via SSE events

### Commands

| Command | Description |
|---------|-------------|
| `/start` | Welcome screen with reply keyboard |
| `/help` | List all available commands |
| `/new` | Start a fresh conversation (new OpenCode session) |
| `/stop` | Abort the current AI operation |
| `/sessions` | List all OpenCode sessions with inline switch buttons |
| `/switch <id>` | Switch to a specific session by ID |
| `/diff` | Show file changes made in the current session |
| `/history` | Show the last 10 messages in the current session |
| `/status` | Bot uptime, active streams, current session info |
| `/stats` | Total messages and session count |
| `/clear` | Delete current session from both bot DB and OpenCode |

### Reply Keyboard

After `/start`, a persistent keyboard appears with quick actions:
- **List files** — asks the AI to list files
- **Docker status** — asks the AI for Docker container status
- **System info** — asks the AI for system information
- **New chat** — starts a fresh conversation

### Security
- **User allowlist** — only authorized Telegram user IDs can interact
- **Rate limiting** — 2-second cooldown between messages per user
- **No hardcoded secrets** — all config via environment variables

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

Create a `.env` file:

```bash
TELEGRAM_BOT_TOKEN=your_bot_token_here
OPENCODE_URL=http://localhost:4096
ALLOWED_USERS=your_telegram_user_id
```

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TELEGRAM_BOT_TOKEN` | Yes | — | Telegram bot token from BotFather |
| `OPENCODE_URL` | No | `http://localhost:4096` | OpenCode server URL |
| `ALLOWED_USERS` | No | — (allow all) | Comma-separated Telegram user IDs |
| `ADMIN_USERS` | No | — | Comma-separated admin user IDs |
| `WORK_DIR` | No | `/home/khale` | Working directory for OpenCode |

To find your Telegram user ID, send a message to [@userinfobot](https://t.me/userinfobot).

### 4. Build

```bash
CGO_ENABLED=1 go build -o opencode-bot .
```

### 5. Run

```bash
./opencode-bot
```

## Systemd Service (Production)

Create `/etc/systemd/system/opencode-go-bot.service`:

```ini
[Unit]
Description=OpenCode Telegram Bot (Go)
After=network.target

[Service]
Type=simple
User=your_user
WorkingDirectory=/path/to/opencode-bot-go
EnvironmentFile=/path/to/opencode-bot-go/.env
ExecStart=/path/to/opencode-bot-go/opencode-bot
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable opencode-go-bot
sudo systemctl start opencode-go-bot

# Check logs
journalctl -u opencode-go-bot -f
```

## How It Works

### Message Flow

1. User sends a text message in Telegram
2. Bot checks auth and rate limit
3. Bot looks up or creates an OpenCode session for this user
4. Bot sends a placeholder "Thinking..." message
5. Bot registers the session with the SSE StreamManager
6. Bot calls `POST /session/:id/prompt_async` (returns immediately)
7. SSE events arrive:
   - `message.part.updated` (type: reasoning) → shows "Thinking..."
   - `message.part.delta` (text deltas) → edits message with streaming text
   - `message.part.updated` (type: text) → sets final text
   - `message.updated` (finish: stop) → cleanup
8. User sees the response build up in real-time

### Session Management

Each Telegram user gets mapped to an OpenCode session. Sessions persist until the user runs `/new` or `/clear`.

- `/new` — deletes the mapping, next message creates a fresh session
- `/switch` — changes the mapping to a different existing session
- `/sessions` — queries `GET /session` from OpenCode API and displays all sessions
- `/clear` — deletes both the local mapping AND the OpenCode session

### SSE Event Handling

The bot maintains a persistent connection to `GET /event` on the OpenCode server. Events are filtered by session ID and routed to the correct Telegram chat.

Key events handled:
- `message.part.delta` — incremental text tokens (streaming)
- `message.part.updated` — full part snapshots (text, reasoning, tool calls)
- `message.updated` — message completion detection
- Reasoning part deltas are filtered out to keep responses clean

### Edit Throttling

Telegram limits message edits. The bot throttles edits to 1 per second per chat to avoid API errors.

## OpenCode API Endpoints Used

| Method | Endpoint | Purpose |
|--------|----------|---------|
| `GET` | `/global/health` | Server health check on startup |
| `POST` | `/session` | Create new session |
| `GET` | `/session` | List all sessions |
| `GET` | `/session/:id` | Get session details |
| `DELETE` | `/session/:id` | Delete session |
| `GET` | `/session/:id/message` | Get message history |
| `POST` | `/session/:id/prompt_async` | Send async prompt |
| `POST` | `/session/:id/abort` | Cancel running operation |
| `GET` | `/session/:id/diff` | Get file changes |
| `GET` | `/event` | SSE event stream |

## Dependencies

- [go-telegram/bot](https://github.com/go-telegram/bot) v1.18.0 — Telegram Bot API
- [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) v1.14.34 — SQLite driver (requires CGO)
