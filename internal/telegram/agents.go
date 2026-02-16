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

func defaultAgents() map[string]string {
	return map[string]string{
		"sisyphus": "General coding",
		"oracle":   "Deep analysis",
	}
}

func parseAgents(raw string) map[string]string {
	agents := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		name := strings.TrimSpace(parts[0])
		desc := name
		if len(parts) == 2 {
			desc = strings.TrimSpace(parts[1])
		}
		if name != "" {
			agents[name] = desc
		}
	}
	return agents
}

func (b *Bot) agentCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	parts := strings.Fields(update.Message.Text)

	// Direct agent set: /agent <name>
	if len(parts) >= 2 {
		agentName := parts[1]
		if _, ok := b.Agents[agentName]; !ok {
			tgBot.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   fmt.Sprintf("Unknown agent: %s", agentName),
			})
			return
		}
		b.setAgent(ctx, tgBot, chatID, agentName)
		return
	}

	// Show agent selection keyboard
	var keyboard [][]models.InlineKeyboardButton
	for name, desc := range b.Agents {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: fmt.Sprintf("%s - %s", name, desc), CallbackData: "agent_" + name},
		})
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Select an agent:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
	})
}

func (b *Bot) setAgent(ctx context.Context, tgBot *bot.Bot, chatID int64, agentName string) {
	if b.DB != nil {
		sess, err := b.DB.GetSession(chatID)
		if err == nil {
			sess.Agent = agentName
			sess.LastUsed = time.Now()
			b.DB.SetSession(sess)
		} else {
			// No session yet — store agent preference for next session
			b.DB.SetSession(store.Session{
				ChatID:    chatID,
				Agent:     agentName,
				CreatedAt: time.Now(),
				LastUsed:  time.Now(),
			})
		}
	}

	desc := b.Agents[agentName]
	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("Agent set to: %s (%s)", agentName, desc),
	})
}

func (b *Bot) handleAgentCallback(ctx context.Context, tgBot *bot.Bot, callback *models.CallbackQuery, agentName string) {
	chatID := callback.Message.Message.Chat.ID

	if _, ok := b.Agents[agentName]; !ok {
		tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            "Unknown agent",
		})
		return
	}

	if b.DB != nil {
		sess, err := b.DB.GetSession(chatID)
		if err == nil {
			sess.Agent = agentName
			sess.LastUsed = time.Now()
			b.DB.SetSession(sess)
		} else {
			b.DB.SetSession(store.Session{
				ChatID:    chatID,
				Agent:     agentName,
				CreatedAt: time.Now(),
				LastUsed:  time.Now(),
			})
		}
	}

	desc := b.Agents[agentName]
	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
		Text:            "Agent: " + agentName,
	})

	tgBot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: callback.Message.Message.ID,
		Text:      fmt.Sprintf("Agent set to: %s (%s)", agentName, desc),
	})

	log.Printf("[agentCallback] Chat %d set agent to %s", chatID, agentName)
}
