package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-telegram/bot"
)

var startTime time.Time

func main() {
	startTime = time.Now()

	// 1. Load configuration
	cfg := LoadConfig()
	log.Printf("Loaded config: OpenCode URL=%s, Allowed Users=%d", cfg.OPENCODE_URL, len(cfg.ALLOWED_USERS))

	// 2. Initialize database
	db, err := NewDB("/home/khale/opencode-bot-go/bot.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// 3. Initialize OpenCode client
	client := NewOpenCodeClient(cfg.OPENCODE_URL)

	// 4. Verify OpenCode server health
	if err := client.Health(context.Background()); err != nil {
		log.Printf("Warning: OpenCode server health check failed: %v", err)
	} else {
		log.Println("OpenCode server is healthy")
	}

	// 5. Create bot with handlers
	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
		bot.WithMessageTextHandler("/start", bot.MatchTypeExact, startCommand),
		bot.WithMessageTextHandler("/help", bot.MatchTypeExact, helpCommand),
		bot.WithMessageTextHandler("/new", bot.MatchTypeExact, newCommand),
		bot.WithMessageTextHandler("/status", bot.MatchTypeExact, statusCommand),
		bot.WithMessageTextHandler("/stats", bot.MatchTypeExact, statsCommand),
		bot.WithMessageTextHandler("/stop", bot.MatchTypeExact, stopCommand),
		bot.WithMessageTextHandler("/clear", bot.MatchTypeExact, clearCommand),
		bot.WithMessageTextHandler("/sessions", bot.MatchTypeExact, sessionsCommand),
		bot.WithMessageTextHandler("/switch", bot.MatchTypePrefix, switchCommand),
		bot.WithMessageTextHandler("/diff", bot.MatchTypeExact, diffCommand),
		bot.WithMessageTextHandler("/history", bot.MatchTypeExact, historyCommand),
		bot.WithMessageTextHandler("/model", bot.MatchTypeExact, modelCommand),
		bot.WithMessageTextHandler("/think", bot.MatchTypeExact, thinkCommand),
	}

	b, err := bot.New(cfg.TELEGRAM_BOT_TOKEN, opts...)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// 6. Initialize StreamManager AFTER bot creation
	streamManager := NewStreamManager(cfg.OPENCODE_URL, b)

	// 7. Set handler dependencies
	SetDeps(&handlerDeps{
		config:        cfg,
		client:        client,
		db:            db,
		streamManager: streamManager,
		startTime:     startTime,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 8. Start SSE stream in goroutine
	go func() {
		if err := streamManager.Start(ctx); err != nil {
			log.Printf("SSE stream error: %v", err)
		}
	}()

	// 9. Start rate limit cleanup
	go cleanupRateLimitMap()

	// 10. Start bot
	log.Println("🤖 OpenCode Bot started (Production Mode - REST API)")
	b.Start(ctx)
}
