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

func (b *Bot) modelCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	parts := strings.Fields(update.Message.Text)

	if len(parts) >= 2 {
		providerModel := parts[1]
		modelParts := strings.SplitN(providerModel, "/", 2)
		if len(modelParts) == 2 {
			b.setModel(ctx, tgBot, chatID, modelParts[0], modelParts[1])
			return
		}
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Invalid format. Use /model provider/model (e.g., /model openai/gpt-4)",
		})
		return
	}

	if len(b.Providers) == 0 {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "No providers available. Check OpenCode server connection.",
		})
		return
	}

	var keyboard [][]models.InlineKeyboardButton
	for _, p := range b.Providers {
		for _, m := range p.Models {
			keyboard = append(keyboard, []models.InlineKeyboardButton{
				{Text: fmt.Sprintf("%s (%s)", m.Name, p.ID), CallbackData: "model_" + p.ID + "/" + m.ID},
			})
		}
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Select a model:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
	})
}

func (b *Bot) setModel(ctx context.Context, tgBot *bot.Bot, chatID int64, providerID, modelID string) {
	if b.DB != nil {
		sess, err := b.DB.GetSession(chatID)
		if err == nil {
			sess.ModelProvider = providerID
			sess.ModelID = modelID
			sess.LastUsed = time.Now()
			b.DB.SetSession(sess)
		} else {
			b.DB.SetSession(store.Session{
				ChatID:        chatID,
				ModelProvider: providerID,
				ModelID:       modelID,
				CreatedAt:     time.Now(),
				LastUsed:      time.Now(),
			})
		}
	}

	displayName := b.findModelDisplayName(providerID, modelID)
	if displayName == "" {
		displayName = providerID + "/" + modelID
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("Model set to: %s", displayName),
	})
}

func (b *Bot) findModelDisplayName(providerID, modelID string) string {
	for _, p := range b.Providers {
		if p.ID == providerID {
			if m, ok := p.Models[modelID]; ok {
				return m.Name + " (" + providerID + ")"
			}
		}
	}
	return ""
}

func (b *Bot) handleModelCallback(ctx context.Context, tgBot *bot.Bot, callback *models.CallbackQuery, providerID, modelID string) {
	chatID := callback.Message.Message.Chat.ID

	displayName := b.findModelDisplayName(providerID, modelID)
	if displayName == "" {
		displayName = providerID + "/" + modelID
	}

	if b.DB != nil {
		sess, err := b.DB.GetSession(chatID)
		if err == nil {
			sess.ModelProvider = providerID
			sess.ModelID = modelID
			sess.LastUsed = time.Now()
			b.DB.SetSession(sess)
		} else {
			b.DB.SetSession(store.Session{
				ChatID:        chatID,
				ModelProvider: providerID,
				ModelID:       modelID,
				CreatedAt:     time.Now(),
				LastUsed:      time.Now(),
			})
		}
	}

	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
		Text:            "Model: " + modelID,
	})

	tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: callback.Message.Message.ID,
		Text:      fmt.Sprintf("Model set to: %s", displayName),
	})

	log.Printf("[modelCallback] Chat %d set model to %s/%s", chatID, providerID, modelID)
}
