package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type handlerDeps struct {
	config        *Config
	client        *OpenCodeClient
	db            *DB
	streamManager *StreamManager
	startTime     time.Time
}

var deps *handlerDeps

func SetDeps(d *handlerDeps) {
	deps = d
}

func requireAuth(chatID int64, b *bot.Bot, ctx context.Context) bool {
	if deps.config == nil {
		return true
	}
	if !checkAuth(chatID, deps.config) {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Unauthorized. You are not allowed to use this bot.",
		})
		return false
	}
	return true
}

func startCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if !requireAuth(chatID, b, ctx) {
		return
	}

	if deps.db != nil {
		if err := deps.db.DeleteSession(chatID); err != nil {
			log.Printf("[startCommand] Error deleting session: %v", err)
		}
	}

	keyboard := [][]models.KeyboardButton{
		{{Text: "📁 List files"}, {Text: "🐳 Docker status"}},
		{{Text: "📋 System info"}, {Text: "🆕 New chat"}},
	}

	helpText := `🤖 *OpenCode Bot*

Connected to OpenCode AI. Conversations preserved!

*Commands:*
/start - Start fresh
/help - Show commands
/new - New conversation
/sessions - List sessions
/diff - Show current changes
/history - Show message history
/stop - Stop current operation
/status - Bot status
/stats - Usage statistics
/clear - Clear all data`

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   helpText,
		ReplyMarkup: &models.ReplyKeyboardMarkup{
			Keyboard:        keyboard,
			ResizeKeyboard:  true,
			OneTimeKeyboard: false,
		},
		ParseMode: models.ParseModeMarkdown,
	})
}

func helpCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if !requireAuth(chatID, b, ctx) {
		return
	}

	helpText := `📚 *Available Commands*

*Basic:*
/start - Start fresh
/help - Show this help
/new - Start new conversation
/stop - Stop current operation

*Session:*
/sessions - List all sessions
/switch <id> - Switch to session
/diff - Show changes
/history - Show messages
/model - Show current model
/think - Toggle thinking display

*Info:*
/status - Bot status
/stats - Usage statistics
/clear - Clear all data`

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      helpText,
		ParseMode: models.ParseModeMarkdown,
	})
}

func newCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if !requireAuth(chatID, b, ctx) {
		return
	}

	if deps.db != nil {
		if err := deps.db.DeleteSession(chatID); err != nil {
			log.Printf("[newCommand] Error deleting session: %v", err)
		}
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "✨ *New conversation started!*",
		ParseMode: models.ParseModeMarkdown,
	})
}

func stopCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if !requireAuth(chatID, b, ctx) {
		return
	}

	var sessionID string
	if deps.db != nil {
		session, err := deps.db.GetSession(chatID)
		if err == nil {
			sessionID = session.SessionID
		}
	}

	if sessionID != "" && deps.client != nil {
		if err := deps.client.Abort(ctx, sessionID); err != nil {
			log.Printf("[stopCommand] Error aborting session %s: %v", sessionID, err)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "⚠️ Error stopping operation",
			})
			return
		}
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "⏹️ *Stopped*",
		ParseMode: models.ParseModeMarkdown,
	})
}

func sessionsCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if !requireAuth(chatID, b, ctx) {
		return
	}

	if deps.client == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ OpenCode client not initialized",
		})
		return
	}

	sessions, err := deps.client.ListOCSessions(ctx)
	if err != nil {
		log.Printf("[sessionsCommand] Error listing sessions: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to fetch sessions",
		})
		return
	}

	if len(sessions) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "📭 No sessions found",
		})
		return
	}

	var currentSessionID string
	if deps.db != nil {
		session, err := deps.db.GetSession(chatID)
		if err == nil {
			currentSessionID = session.SessionID
		}
	}

	var sb strings.Builder
	sb.WriteString("📋 *Available Sessions*\n\n")

	var keyboard [][]models.InlineKeyboardButton

	for i, sess := range sessions {
		title := sess.Title
		if title == "" {
			title = "Untitled"
		}

		indicator := ""
		if sess.ID == currentSessionID {
			indicator = " ✅"
		}

		sb.WriteString(fmt.Sprintf("%d. `%s` - %s%s\n", i+1, sess.ID, title, indicator))

		callbackData := fmt.Sprintf("switch_%s", sess.ID)
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: fmt.Sprintf("Switch to %s", sess.ID[:8]+"..."), CallbackData: callbackData},
		})
	}

	sb.WriteString("\nUse /switch <id> to switch sessions")

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   sb.String(),
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
		ParseMode: models.ParseModeMarkdown,
	})
}

func switchCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if !requireAuth(chatID, b, ctx) {
		return
	}

	text := update.Message.Text

	parts := strings.Fields(text)
	if len(parts) < 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Usage: /switch <session_id>",
		})
		return
	}

	sessionID := parts[1]

	if deps.client != nil {
		_, err := deps.client.GetOCSession(ctx, sessionID)
		if err != nil {
			log.Printf("[switchCommand] Session not found: %s", sessionID)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Session not found",
			})
			return
		}
	}

	if deps.db != nil {
		session := Session{
			ChatID:    chatID,
			SessionID: sessionID,
			Title:     "",
			LastUsed:  time.Now(),
		}
		if err := deps.db.SetSession(session); err != nil {
			log.Printf("[switchCommand] Error saving session: %v", err)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Failed to save session",
			})
			return
		}
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("🔄 Switched to session: `%s`", sessionID),
		ParseMode: models.ParseModeMarkdown,
	})
}

func diffCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if !requireAuth(chatID, b, ctx) {
		return
	}

	var sessionID string
	if deps.db != nil {
		session, err := deps.db.GetSession(chatID)
		if err != nil {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ No active session. Send a message first.",
			})
			return
		}
		sessionID = session.SessionID
	}

	if sessionID == "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ No active session",
		})
		return
	}

	if deps.client == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ OpenCode client not initialized",
		})
		return
	}

	diff, err := deps.client.GetDiff(ctx, sessionID)
	if err != nil {
		log.Printf("[diffCommand] Error getting diff: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to get diff",
		})
		return
	}

	if diff == "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "📭 No changes",
		})
		return
	}

	if len(diff) > 4000 {
		diff = diff[:4000] + "\n\n_... (truncated)_"
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      "📝 *Current Changes*\n\n```\n" + diff + "\n```",
		ParseMode: models.ParseModeMarkdown,
	})
}

func historyCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if !requireAuth(chatID, b, ctx) {
		return
	}

	var sessionID string
	if deps.db != nil {
		session, err := deps.db.GetSession(chatID)
		if err != nil {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ No active session. Send a message first.",
			})
			return
		}
		sessionID = session.SessionID
	}

	if sessionID == "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ No active session",
		})
		return
	}

	if deps.client == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ OpenCode client not initialized",
		})
		return
	}

	messages, err := deps.client.GetMessages(ctx, sessionID)
	if err != nil {
		log.Printf("[historyCommand] Error getting messages: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to get history",
		})
		return
	}

	if len(messages) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "📭 No messages yet",
		})
		return
	}

	var sb strings.Builder
	sb.WriteString("💬 *Recent Messages*\n\n")

	start := 0
	if len(messages) > 10 {
		start = len(messages) - 10
	}

	for i := start; i < len(messages); i++ {
		msg := messages[i]
		role := msg.Role
		if role == "" {
			role = "user"
		}

		content := msg.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}

		sb.WriteString(fmt.Sprintf("*%s:*\n%s\n\n", role, content))
	}

	text := sb.String()
	if len(text) > 4000 {
		text = text[:4000] + "\n_... (truncated)_"
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
}

func modelCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "🤖 *Model:* default (OpenCode model)",
		ParseMode: models.ParseModeMarkdown,
	})
}

func thinkCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "🧠 *Thinking display:* ON",
		ParseMode: models.ParseModeMarkdown,
	})
}

func statusCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if !requireAuth(chatID, b, ctx) {
		return
	}

	uptime := time.Since(deps.startTime)

	var sessionInfo string
	if deps.db != nil {
		session, err := deps.db.GetSession(chatID)
		if err == nil {
			sessionInfo = fmt.Sprintf("\nSession: `%s`\nMessages: %d", session.SessionID, session.MessageCount)
		}
	}

	activeStreams := 0
	if deps.streamManager != nil {
		activeStreams = deps.streamManager.GetActiveSessionCount()
	}

	statusText := fmt.Sprintf("✅ *Bot Status*\n\nUptime: %s\nActive streams: %d%s",
		uptime.Round(time.Second),
		activeStreams,
		sessionInfo,
	)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      statusText,
		ParseMode: models.ParseModeMarkdown,
	})
}

func statsCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if !requireAuth(chatID, b, ctx) {
		return
	}

	if deps.db == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Database not initialized",
		})
		return
	}

	sessions, err := deps.db.ListAll()
	if err != nil {
		log.Printf("[statsCommand] Error listing sessions: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Failed to get statistics",
		})
		return
	}

	totalMessages := 0
	for _, sess := range sessions {
		totalMessages += sess.MessageCount
	}

	statsText := fmt.Sprintf("📊 *Statistics*\n\nTotal messages: %d\nActive sessions: %d",
		totalMessages,
		len(sessions),
	)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      statsText,
		ParseMode: models.ParseModeMarkdown,
	})
}

func clearCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if !requireAuth(chatID, b, ctx) {
		return
	}

	var sessionID string
	if deps.db != nil {
		session, err := deps.db.GetSession(chatID)
		if err == nil {
			sessionID = session.SessionID
		}
	}

	if deps.db != nil {
		if err := deps.db.DeleteSession(chatID); err != nil {
			log.Printf("[clearCommand] Error deleting session from DB: %v", err)
		}
	}

	if sessionID != "" && deps.client != nil {
		if err := deps.client.DeleteOCSession(ctx, sessionID); err != nil {
			log.Printf("[clearCommand] Error deleting session from OpenCode: %v", err)
		}
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "🗑️ *Data cleared!*",
		ParseMode: models.ParseModeMarkdown,
	})
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Handle callback queries (inline keyboard buttons)
	if update.CallbackQuery != nil {
		handleCallbackQuery(ctx, b, update)
		return
	}

	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	text := update.Message.Text

	if text == "" {
		return
	}

	if deps.config != nil && !checkAuth(chatID, deps.config) {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Unauthorized. You are not allowed to use this bot.",
		})
		return
	}

	if !checkRateLimit(chatID) {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "⏳ Please wait a moment before sending another message...",
		})
		return
	}

	b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID: chatID,
		Action: "typing",
	})

	var sessionID string

	if deps.db != nil {
		session, err := deps.db.GetSession(chatID)
		if err == nil {
			sessionID = session.SessionID
			deps.db.IncrementCount(chatID)
		}
	}

	if sessionID == "" && deps.client != nil {
		newSession, err := deps.client.CreateOCSession(ctx, fmt.Sprintf("Telegram Chat %d", chatID))
		if err != nil {
			log.Printf("[defaultHandler] Error creating session: %v", err)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Failed to create session: " + err.Error(),
			})
			return
		}

		sessionID = newSession.ID

		if deps.db != nil {
			session := Session{
				ChatID:       chatID,
				SessionID:    sessionID,
				Title:        newSession.Title,
				MessageCount: 1,
				CreatedAt:    time.Now(),
				LastUsed:     time.Now(),
			}
			if err := deps.db.SetSession(session); err != nil {
				log.Printf("[defaultHandler] Error saving session: %v", err)
			}
		}
	}

	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "🤔 Thinking...",
	})
	if err != nil {
		log.Printf("[defaultHandler] Error sending initial message: %v", err)
		return
	}

	if deps.streamManager != nil && sessionID != "" {
		deps.streamManager.RegisterSession(sessionID, chatID, msg.ID)
	}

	if deps.client != nil && sessionID != "" {
		if err := deps.client.PromptAsync(ctx, sessionID, text); err != nil {
			log.Printf("[defaultHandler] Error sending prompt: %v", err)
			b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    chatID,
				MessageID: msg.ID,
				Text:      "❌ Error: " + err.Error(),
			})
			return
		}
	} else {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: msg.ID,
			Text:      "❌ OpenCode client not available",
		})
	}
}

func handleCallbackQuery(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	callback := update.CallbackQuery
	chatID := callback.Message.Message.Chat.ID
	data := callback.Data

	if strings.HasPrefix(data, "switch_") {
		sessionID := strings.TrimPrefix(data, "switch_")

		if deps.client != nil {
			_, err := deps.client.GetOCSession(ctx, sessionID)
			if err != nil {
				b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
					CallbackQueryID: callback.ID,
					Text:           "Session not found",
				})
				return
			}
		}

		if deps.db != nil {
			session := Session{
				ChatID:    chatID,
				SessionID: sessionID,
				LastUsed:  time.Now(),
			}
			if err := deps.db.SetSession(session); err != nil {
				log.Printf("[handleCallbackQuery] Error saving session: %v", err)
			}
		}

		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Switched to " + sessionID[:8] + "...",
		})

		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: callback.Message.Message.ID,
			Text:      fmt.Sprintf("🔄 Switched to session: `%s`", sessionID),
			ParseMode: models.ParseModeMarkdown,
		})
	}
}

func isCommand(text string) bool {
	commands := []string{
		"/start", "/help", "/new", "/stop", "/sessions",
		"/switch", "/diff", "/history", "/model", "/think",
		"/status", "/stats", "/clear",
	}

	for _, cmd := range commands {
		if strings.HasPrefix(text, cmd) {
			return true
		}
	}
	return false
}
