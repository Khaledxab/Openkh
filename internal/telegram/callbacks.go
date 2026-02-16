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

func (b *Bot) defaultHandler(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.CallbackQuery != nil {
		b.handleCallbackQuery(ctx, tgBot, update)
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

	if b.Config != nil && !checkAuth(chatID, b.Config) {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Unauthorized. You are not allowed to use this bot.",
		})
		return
	}

	if !checkRateLimit(chatID) {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Please wait a moment before sending another message...",
		})
		return
	}

	tgBot.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID: chatID,
		Action: "typing",
	})

	var sessionID string
	var agent string

	if b.DB != nil {
		sess, err := b.DB.GetSession(chatID)
		if err == nil {
			sessionID = sess.SessionID
			agent = sess.Agent
			b.DB.IncrementCount(chatID)
		}
	}

	if agent == "" {
		agent = "sisyphus"
	}

	if sessionID == "" && b.Client != nil {
		newSess, err := b.Client.CreateOCSession(ctx, fmt.Sprintf("Telegram Chat %d", chatID))
		if err != nil {
			log.Printf("[defaultHandler] Error creating session: %v", err)
			tgBot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Failed to create session: " + err.Error(),
			})
			return
		}
		sessionID = newSess.ID

		if b.DB != nil {
			s := store.Session{
				ChatID:       chatID,
				SessionID:    sessionID,
				Title:        newSess.Title,
				Agent:        agent,
				MessageCount: 1,
				CreatedAt:    time.Now(),
				LastUsed:     time.Now(),
			}
			if err := b.DB.SetSession(s); err != nil {
				log.Printf("[defaultHandler] Error saving session: %v", err)
			}
		}
	}

	msg, err := tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Thinking...",
	})
	if err != nil {
		log.Printf("[defaultHandler] Error sending initial message: %v", err)
		return
	}

	if b.Stream != nil && sessionID != "" {
		b.Stream.RegisterSession(sessionID, chatID, msg.ID)
	}

	if b.Client != nil && sessionID != "" {
		if err := b.Client.PromptAsync(ctx, sessionID, text, agent); err != nil {
			log.Printf("[defaultHandler] Error sending prompt: %v", err)
			tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    chatID,
				MessageID: msg.ID,
				Text:      "Error: " + err.Error(),
			})
			return
		}
	} else {
		tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: msg.ID,
			Text:      "OpenCode client not available",
		})
	}
}

func (b *Bot) handleCallbackQuery(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}
	callback := update.CallbackQuery
	chatID := callback.Message.Message.Chat.ID
	data := callback.Data

	if strings.HasPrefix(data, "switch_") {
		sessionID := strings.TrimPrefix(data, "switch_")
		b.handleSwitchCallback(ctx, tgBot, callback, chatID, sessionID)
		return
	}

	if strings.HasPrefix(data, "agent_") {
		agentName := strings.TrimPrefix(data, "agent_")
		b.handleAgentCallback(ctx, tgBot, callback, agentName)
		return
	}
}

func (b *Bot) handleSwitchCallback(ctx context.Context, tgBot *bot.Bot, callback *models.CallbackQuery, chatID int64, sessionID string) {
	if b.Client != nil {
		if _, err := b.Client.GetOCSession(ctx, sessionID); err != nil {
			tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: callback.ID,
				Text:            "Session not found",
			})
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
			log.Printf("[handleSwitchCallback] Error: %v", err)
		}
	}

	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
		Text:            "Switched to " + shortID(sessionID),
	})

	tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: callback.Message.Message.ID,
		Text:      fmt.Sprintf("Switched to session: %s", shortID(sessionID)),
	})
}
