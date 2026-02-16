package telegram

import (
	"context"
	"log"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) startCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	if b.DB != nil {
		if err := b.DB.DeleteSession(chatID); err != nil {
			log.Printf("[startCommand] Error deleting session: %v", err)
		}
	}

	keyboard := [][]models.KeyboardButton{
		{{Text: "List files"}, {Text: "Docker status"}},
		{{Text: "System info"}, {Text: "New chat"}},
	}

	helpText := "OpenCode Bot\n\nConnected to OpenCode AI. Conversations preserved!\n\nCommands:\n" +
		"/start - Start fresh\n/help - Show commands\n/new - New conversation\n" +
		"/sessions - List sessions\n/agent - Switch agent\n/rename - Rename session\n" +
		"/delete - Delete session\n/purge - Delete all sessions\n" +
		"/diff - Show current changes\n/history - Show message history\n" +
		"/stop - Stop current operation\n/status - Bot status\n/stats - Usage statistics\n" +
		"/clear - Clear current session"

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   helpText,
		ReplyMarkup: &models.ReplyKeyboardMarkup{
			Keyboard:        keyboard,
			ResizeKeyboard:  true,
			OneTimeKeyboard: false,
		},
	})
}

func (b *Bot) helpCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	helpText := "Available Commands\n\n" +
		"Basic:\n/start - Start fresh\n/help - Show this help\n/new - New conversation\n/stop - Stop current operation\n\n" +
		"Session:\n/sessions - List all sessions\n/switch <id> - Switch to session\n/rename <title> - Rename session\n/delete <id> - Delete session\n/purge - Delete all sessions\n\n" +
		"Agent:\n/agent - Switch agent\n/agent <name> - Set agent directly\n\n" +
		"Tools:\n/diff - Show changes\n/history - Show messages\n/model - Show current model\n/think - Toggle thinking display\n\n" +
		"Info:\n/status - Bot status\n/stats - Usage statistics\n/clear - Clear current session"

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   helpText,
	})
}

func (b *Bot) newCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	if b.DB != nil {
		if err := b.DB.DeleteSession(chatID); err != nil {
			log.Printf("[newCommand] Error deleting session: %v", err)
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "New conversation started!",
	})
}

func (b *Bot) stopCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	var sessionID string
	if b.DB != nil {
		sess, err := b.DB.GetSession(chatID)
		if err == nil {
			sessionID = sess.SessionID
		}
	}

	if sessionID != "" && b.Client != nil {
		if err := b.Client.Abort(ctx, sessionID); err != nil {
			log.Printf("[stopCommand] Error aborting session %s: %v", sessionID, err)
			tgBot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Error stopping operation",
			})
			return
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Stopped",
	})
}

func (b *Bot) clearCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	var sessionID string
	if b.DB != nil {
		sess, err := b.DB.GetSession(chatID)
		if err == nil {
			sessionID = sess.SessionID
		}
	}

	if b.DB != nil {
		if err := b.DB.DeleteSession(chatID); err != nil {
			log.Printf("[clearCommand] Error deleting session: %v", err)
		}
	}

	if sessionID != "" && b.Client != nil {
		if err := b.Client.DeleteOCSession(ctx, sessionID); err != nil {
			log.Printf("[clearCommand] Error deleting OC session: %v", err)
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Data cleared!",
	})
}

func (b *Bot) modelCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Model: default (OpenCode model)",
	})
}

func (b *Bot) thinkCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Thinking display: ON",
	})
}
