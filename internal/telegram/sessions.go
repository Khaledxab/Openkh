package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Khaledxab/Openkh/internal/store"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) sessionsCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	log.Printf("[sessionsCommand] Called")
	if update.Message == nil {
		log.Printf("[sessionsCommand] update.Message is nil")
		return
	}
	chatID := update.Message.Chat.ID
	log.Printf("[sessionsCommand] chatID=%d", chatID)
	if !b.requireAuth(chatID, tgBot, ctx) {
		log.Printf("[sessionsCommand] requireAuth returned false")
		return
	}
	log.Printf("[sessionsCommand] auth passed, Client=%v", b.Client)

	log.Printf("[sessionsCommand] Calling ListOCSessions...")
	sessions, err := b.Client.ListOCSessions(ctx)
	log.Printf("[sessionsCommand] ListOCSessions returned, err=%v, sessions=%d", err, len(sessions))
	
	if len(sessions) == 0 {
		log.Printf("[sessionsCommand] No sessions, sending message")
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "No sessions found"})
		return
	}

	totalSessions := len(sessions)
	log.Printf("[sessionsCommand] Building response for %d sessions", totalSessions)

	var currentSessionID string
	if b.DB != nil {
		sess, err := b.DB.GetSession(chatID)
		if err == nil {
			currentSessionID = sess.SessionID
		}
	}
	log.Printf("[sessionsCommand] Got current session: %s", currentSessionID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Available Sessions (%d total, showing first %d)\n\n", totalSessions, len(sessions)))

	var keyboard [][]models.InlineKeyboardButton
	log.Printf("[sessionsCommand] Starting loop over sessions")
	
	// Limit to 20 sessions max to avoid message too long error
	maxSessions := 20
	if len(sessions) > maxSessions {
		sessions = sessions[:maxSessions]
	}
	
	for i, sess := range sessions {
		title := sess.Title
		if title == "" {
			title = "Untitled"
		}
		indicator := ""
		if sess.ID == currentSessionID {
			indicator = " [active]"
		}
		sb.WriteString(fmt.Sprintf("%d. %s - %s%s\n", i+1, shortID(sess.ID), title, indicator))

		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: fmt.Sprintf("Switch to %s", shortID(sess.ID)), CallbackData: "switch_" + sess.ID},
		})
		if i == 0 {
			log.Printf("[sessionsCommand] First iteration done")
		}
	}
	log.Printf("[sessionsCommand] Loop done, keyboard size: %d", len(keyboard))
	
	sb.WriteString("\nUse /switch <id> to switch sessions")
	log.Printf("[sessionsCommand] Sending message to chatID=%d, text length=%d", chatID, len(sb.String()))
	
	msg, err := tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   sb.String(),
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
	})
	log.Printf("[sessionsCommand] SendMessage result: msgID=%d, err=%v", msg.ID, err)
}

func (b *Bot) switchCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Usage: /switch <session_id>"})
		return
	}
	sessionID := parts[1]

	if b.Client != nil {
		if _, err := b.Client.GetOCSession(ctx, sessionID); err != nil {
			tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Session not found"})
			return
		}
	}

	if b.DB != nil {
		sess := store.Session{
			ChatID:    chatID,
			SessionID: sessionID,
			LastUsed:  time.Now(),
		}
		if err := b.DB.SetSession(sess); err != nil {
			log.Printf("[switchCommand] Error: %v", err)
			tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Failed to save session"})
			return
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("Switched to session: %s", shortID(sessionID)),
	})
}

func (b *Bot) renameCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	parts := strings.SplitN(update.Message.Text, " ", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Usage: /rename <new title>"})
		return
	}
	newTitle := strings.TrimSpace(parts[1])

	var sessionID string
	if b.DB != nil {
		sess, err := b.DB.GetSession(chatID)
		if err != nil {
			tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "No active session. Send a message first."})
			return
		}
		sessionID = sess.SessionID
	}
	if sessionID == "" {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "No active session"})
		return
	}

	if b.Client != nil {
		if _, err := b.Client.RenameOCSession(ctx, sessionID, newTitle); err != nil {
			log.Printf("[renameCommand] Error: %v", err)
			tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Failed to rename session"})
			return
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("Session renamed to: %s", newTitle),
	})
}

func (b *Bot) deleteCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		// Delete current session
		if b.DB != nil {
			sess, err := b.DB.GetSession(chatID)
			if err != nil {
				tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "No active session to delete"})
				return
			}
			if b.Client != nil {
				if err := b.Client.DeleteOCSession(ctx, sess.SessionID); err != nil {
					log.Printf("[deleteCommand] Error deleting OC session: %v", err)
				}
			}
			if err := b.DB.DeleteSession(chatID); err != nil {
				log.Printf("[deleteCommand] Error: %v", err)
			}
			tgBot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   fmt.Sprintf("Deleted session: %s", shortID(sess.SessionID)),
			})
			return
		}
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Usage: /delete [session_id]"})
		return
	}

	sessionID := parts[1]
	if b.Client != nil {
		if err := b.Client.DeleteOCSession(ctx, sessionID); err != nil {
			log.Printf("[deleteCommand] Error: %v", err)
			tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Failed to delete session"})
			return
		}
	}

	// If deleting the current session, clear the DB mapping too
	if b.DB != nil {
		sess, err := b.DB.GetSession(chatID)
		if err == nil && sess.SessionID == sessionID {
			b.DB.DeleteSession(chatID)
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("Deleted session: %s", shortID(sessionID)),
	})
}

func (b *Bot) purgeCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}
	if !b.isAdmin(chatID) {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Admin only command"})
		return
	}

	// Delete all OC sessions
	if b.Client != nil {
		sessions, err := b.Client.ListOCSessions(ctx)
		if err == nil {
			for _, sess := range sessions {
				if err := b.Client.DeleteOCSession(ctx, sess.ID); err != nil {
					log.Printf("[purgeCommand] Error deleting OC session %s: %v", shortID(sess.ID), err)
				}
			}
		}
	}

	// Clear all DB mappings
	if b.DB != nil {
		if err := b.DB.DeleteAll(); err != nil {
			log.Printf("[purgeCommand] Error clearing DB: %v", err)
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "All sessions purged!",
	})
}

func (b *Bot) diffCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	sessionID := b.currentSessionID(chatID)
	if sessionID == "" {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "No active session. Send a message first."})
		return
	}
	if b.Client == nil {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "OpenCode client not initialized"})
		return
	}

	diff, err := b.Client.GetDiff(ctx, sessionID)
	if err != nil {
		log.Printf("[diffCommand] Error: %v", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Failed to get diff"})
		return
	}
	if diff == "" {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "No changes"})
		return
	}
	if len(diff) > 4000 {
		diff = diff[:4000] + "\n\n... (truncated)"
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Current Changes\n\n" + diff,
	})
}

func (b *Bot) historyCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	sessionID := b.currentSessionID(chatID)
	if sessionID == "" {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "No active session. Send a message first."})
		return
	}
	if b.Client == nil {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "OpenCode client not initialized"})
		return
	}

	messages, err := b.Client.GetMessages(ctx, sessionID)
	if err != nil {
		log.Printf("[historyCommand] Error: %v", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Failed to get history"})
		return
	}
	if len(messages) == 0 {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "No messages yet"})
		return
	}

	var sb strings.Builder
	sb.WriteString("Recent Messages\n\n")

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
		sb.WriteString(fmt.Sprintf("%s:\n%s\n\n", role, content))
	}

	text := sb.String()
	if len(text) > 4000 {
		text = text[:4000] + "\n... (truncated)"
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
}
