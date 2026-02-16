# AGENTS.md

Guidelines for AI coding agents working in this repository.

## Project Overview

This repository contains a Telegram bot that bridges to an OpenCode AI server. The main project is `opencode-bot-go/` - a Go application.

## Build, Test & Lint Commands

### Go Project (opencode-bot-go)

```bash
# Navigate to project
cd opencode-bot-go

# Build (CGO required for mattn/go-sqlite3)
CGO_ENABLED=1 go build -o bin/openkh ./cmd/openkh

# Or use Makefile
make build    # builds to bin/openkh
make run      # build + run
make test     # go test ./...
make lint     # go vet ./...
make clean    # remove bin/

# Run single test
go test -v ./internal/store/...
go test -v ./internal/opencode/...
go test -v ./internal/telegram/...
go test -run TestFunctionName ./...

# Deploy (kills old process, sources .env, starts in background)
bash start.sh
```

### Static Site (openkh)

```bash
cd openkh
docker-compose up --build
```

## Code Style Guidelines

### Go Conventions

- **Go Version**: 1.21.5
- **CGO**: Mandatory (required for `mattn/go-sqlite3`)
- **Formatting**: Use `go fmt` before commits
- **Linting**: Run `go vet ./...` before commits

### Naming Conventions

- **Files**: `snake_case.go` (e.g., `bot.go`, `stream_manager.go`)
- **Types/Interfaces**: `PascalCase` (e.g., `Bot`, `MessageSender`)
- **Functions/Variables**: `camelCase` (e.g., `sendText`, `streamMgr`)
- **Constants**: `PascalCase` or `SnakeCase` (e.g., `MaxRetries`, `max_retries`)
- **Interfaces**: Name with `er` suffix when simple (e.g., `Reader`, `Sender`)

### Package Structure

```
internal/
├── config/    # Environment-based configuration
├── store/     # SQLite database operations
├── opencode/  # OpenCode API client (HTTP + SSE)
└── telegram/  # Telegram bot handlers
```

### Import Organization

Group imports in this order (blank line between groups):
1. Standard library
2. External packages
3. Internal packages

```go
import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/go-telegram/bot"
    "github.com/mattn/go-sqlite3"

    "github.com/Khaledxab/Openkh/internal/config"
    "github.com/Khaledxab/Openkh/internal/store"
)
```

### Error Handling

- Return errors with context using `fmt.Errorf("description: %w", err)`
- Never silently ignore errors with `_`
- Handle errors at the call site
- Use sentinel errors for known conditions

```go
// Good
if err != nil {
    return fmt.Errorf("failed to process: %w", err)
}

// Bad
_, _ = someFunc() // Never do this
```

### Interfaces

Define interfaces close to their usage. Prefer small, focused interfaces.

```go
// Defined in internal/opencode (no Telegram dependency)
type MessageSender interface {
    SendText(chatID int64, text string) error
    EditText(chatID int64, messageID int, text string) error
}
```

### Goroutines & Concurrency

- Always provide cancellation via `context.Context`
- Use `sync.WaitGroup` for coordinating multiple goroutines
- Channel ownership: sender closes, not receiver

### Database

- Use SQLite with `mattn/go-sqlite3`
- Auto-migrate schema on startup
- Use prepared statements for queries

### Configuration

- Use `.env` file (no `export` prefix - plain `KEY=VALUE`)
- XDG-compliant paths: `$DB_PATH` > `$DATA_DIR/openkh.db` > `$XDG_DATA_HOME/openkh/` > `~/.local/share/openkh/`

### Logging

- Use standard `log` package for simplicity
- Write to `bot.log` in project root

## Architecture

### Dependency Wiring

The bot uses two-phase initialization in `cmd/openkh/main.go`:
1. Create Bot with `Stream: nil`
2. Create Telegram library bot
3. Wrap as `MessageSender` adapter
4. Create StreamManager
5. Inject stream back into handlers

### SSE Streaming Flow

1. Handler sends "Thinking..." message
2. `client.PromptAsync()` fires prompt (returns immediately)
3. Background SSE receives `message.part.delta` events
4. `editMessage()` updates Telegram message (throttled to 1/sec)
5. `message.updated` with `finish != ""` triggers completion

### Agent System

- Default agents: `sisyphus` (general coding), `oracle` (deep analysis)
- Override via `AGENTS` env var: `name:desc,name:desc`
- Agent name persisted per-chat in SQLite

## Environment Variables

Create `.env` from `.env.example`:

```
TELEGRAM_BOT_TOKEN=your_token_here
OPENCODE_URL=http://localhost:4096
```

## Testing

- Write tests in `*_test.go` files alongside source
- Use table-driven tests for multiple test cases
- Mock external dependencies

```go
func TestProcess(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid", "hello", "hello", false},
        {"empty", "", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := process(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("process() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("process() = %v, want %v", got, tt.want)
            }
        })
    }
}
```
