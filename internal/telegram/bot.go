package telegram

import (
	"context"
	"log"
	"time"

	"github.com/Khaledxab/Openkh/internal/config"
	"github.com/Khaledxab/Openkh/internal/opencode"
	"github.com/Khaledxab/Openkh/internal/store"
	"github.com/go-telegram/bot"
)

// Bot holds all dependencies and registers handlers.
type Bot struct {
	Config *config.Config
	Client *opencode.Client
	DB     *store.DB
	Stream *opencode.StreamManager
	Start  time.Time
	Agents map[string]string // name -> description
}

// New creates a Bot and initialises the agent map.
func New(cfg *config.Config, client *opencode.Client, db *store.DB, stream *opencode.StreamManager) *Bot {
	b := &Bot{
		Config: cfg,
		Client: client,
		DB:     db,
		Stream: stream,
		Start:  time.Now(),
		Agents: defaultAgents(),
	}

	// Override with env-configured agents if provided
	if cfg.Agents != "" {
		if parsed := parseAgents(cfg.Agents); len(parsed) > 0 {
			b.Agents = parsed
		}
	}

	return b
}

// RegisterHandlers returns the bot.Option slice for all command/handler registrations.
func (b *Bot) RegisterHandlers() []bot.Option {
	return []bot.Option{
		bot.WithDefaultHandler(b.defaultHandler),
		bot.WithMessageTextHandler("/start", bot.MatchTypeExact, b.startCommand),
		bot.WithMessageTextHandler("/help", bot.MatchTypeExact, b.helpCommand),
		bot.WithMessageTextHandler("/new", bot.MatchTypeExact, b.newCommand),
		bot.WithMessageTextHandler("/status", bot.MatchTypeExact, b.statusCommand),
		bot.WithMessageTextHandler("/stats", bot.MatchTypeExact, b.statsCommand),
		bot.WithMessageTextHandler("/stop", bot.MatchTypeExact, b.stopCommand),
		bot.WithMessageTextHandler("/clear", bot.MatchTypeExact, b.clearCommand),
		bot.WithMessageTextHandler("/sessions", bot.MatchTypeExact, b.sessionsCommand),
		bot.WithMessageTextHandler("/switch", bot.MatchTypePrefix, b.switchCommand),
		bot.WithMessageTextHandler("/rename", bot.MatchTypePrefix, b.renameCommand),
		bot.WithMessageTextHandler("/delete", bot.MatchTypePrefix, b.deleteCommand),
		bot.WithMessageTextHandler("/purge", bot.MatchTypeExact, b.purgeCommand),
		bot.WithMessageTextHandler("/diff", bot.MatchTypeExact, b.diffCommand),
		bot.WithMessageTextHandler("/history", bot.MatchTypeExact, b.historyCommand),
		bot.WithMessageTextHandler("/model", bot.MatchTypeExact, b.modelCommand),
		bot.WithMessageTextHandler("/think", bot.MatchTypeExact, b.thinkCommand),
		bot.WithMessageTextHandler("/agent", bot.MatchTypePrefix, b.agentCommand),
	}
}

// TelegramSender adapts a *bot.Bot to opencode.MessageSender.
type TelegramSender struct {
	Bot *bot.Bot
}

func (ts *TelegramSender) SendText(chatID int64, text string) (int, error) {
	msg, err := ts.Bot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
}

func (ts *TelegramSender) EditText(chatID int64, messageID int, text string) error {
	_, err := ts.Bot.EditMessageText(context.Background(), &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
	})
	return err
}

// StartRateLimitCleanup runs the periodic rate-limit map cleanup.
func StartRateLimitCleanup() {
	go cleanupRateLimitMap()
}

// LogConfig logs the loaded configuration summary.
func LogConfig(cfg *config.Config) {
	log.Printf("Loaded config: OpenCode URL=%s, Allowed Users=%d, DB=%s",
		cfg.OpenCodeURL, len(cfg.AllowedUsers), cfg.DBPath)
}
