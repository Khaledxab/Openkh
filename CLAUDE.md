# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build (CGO required for mattn/go-sqlite3)
CGO_ENABLED=1 go build -o bin/openkh ./cmd/openkh

# Run
make run

# Lint
go vet ./...

# Test
go test ./...

# Deploy (kills old process, sources .env, starts in background)
bash start.sh
```

CGO is mandatory — the only C dependency is `mattn/go-sqlite3`.

## Architecture

The bot bridges Telegram to an OpenCode AI server (`localhost:4096`). Two runtime loops: Telegram long-polling and SSE streaming from OpenCode.

**Dependency wiring** (`cmd/openkh/main.go`) uses two-phase init:
1. `telegram.New(cfg, client, db, nil)` creates the Bot with `Stream: nil`
2. `bot.New(token, opts...)` creates the Telegram library bot
3. `TelegramSender{Bot: tgBot}` wraps it as a `MessageSender`
4. `opencode.NewStreamManager(url, sender)` creates the stream manager
5. `tgHandler.Stream = stream` injects it back — completing the cycle

This exists because `RegisterHandlers()` must produce `[]bot.Option` before the Telegram bot exists, but `StreamManager` needs the Telegram bot for sending messages.

**Key decoupling:** `opencode.MessageSender` interface (2 methods: `SendText`, `EditText`) keeps the `opencode` package free of any Telegram dependency. `telegram.TelegramSender` is the adapter.

## Package Layout

- **`internal/config`** — env-based config with XDG-compliant DB path resolution (`$DB_PATH` > `$DATA_DIR/openkh.db` > `$XDG_DATA_HOME/openkh/` > `~/.local/share/openkh/`)
- **`internal/store`** — SQLite session mapping (chat_id -> session_id + agent + message_count). Auto-migrates `agent` column on old schemas.
- **`internal/opencode`** — HTTP client for OpenCode REST API + SSE `StreamManager`. Zero Telegram imports.
- **`internal/telegram`** — All handlers are methods on `Bot` struct. Split by domain: `commands.go`, `sessions.go`, `agents.go`, `callbacks.go`, `info.go`, `middleware.go`, `helpers.go`.

## SSE Streaming Flow

1. `defaultHandler` sends "Thinking..." message, calls `stream.RegisterSession(sessionID, chatID, msgID)`
2. `client.PromptAsync()` fires the prompt (returns immediately)
3. Background SSE goroutine receives `message.part.delta` events, appends to accumulated text
4. `editMessage()` updates the Telegram message in-place (throttled to 1 edit/second)
5. `message.updated` with `finish != ""` triggers `markComplete()` — final edit + map cleanup

## Agent System

Default agents: `sisyphus` (General coding), `oracle` (Deep analysis). Override via `AGENTS` env var: `name:desc,name:desc`. Agent name persisted per-chat in SQLite, passed in `PromptAsync` payload.

## .env Format

No `export` prefix — plain `KEY=VALUE` for systemd `EnvironmentFile` compatibility.
