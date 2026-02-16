package telegram

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) statusCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	uptime := time.Since(b.Start)

	var sessionInfo string
	if b.DB != nil {
		sess, err := b.DB.GetSession(chatID)
		if err == nil {
			modelInfo := "server default"
			if sess.ModelProvider != "" && sess.ModelID != "" {
				modelInfo = sess.ModelID + " (" + sess.ModelProvider + ")"
			}
			sessionInfo = fmt.Sprintf("\nSession: %s\nModel: %s\nAgent: %s\nMessages: %d",
				shortID(sess.SessionID), modelInfo, agentOrDefault(sess.Agent), sess.MessageCount)
		}
	}

	activeStreams := 0
	if b.Stream != nil {
		activeStreams = b.Stream.GetActiveSessionCount()
	}

	text := fmt.Sprintf("Bot Status\n\nUptime: %s\nActive streams: %d%s",
		uptime.Round(time.Second), activeStreams, sessionInfo)

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
}

func (b *Bot) statsCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	if !b.requireAuth(chatID, tgBot, ctx) {
		return
	}

	if b.DB == nil {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Database not initialized"})
		return
	}

	sessions, err := b.DB.ListAll()
	if err != nil {
		log.Printf("[statsCommand] Error: %v", err)
		tgBot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Failed to get statistics"})
		return
	}

	totalMessages := 0
	for _, sess := range sessions {
		totalMessages += sess.MessageCount
	}

	text := fmt.Sprintf("Statistics\n\nTotal messages: %d\nActive sessions: %d",
		totalMessages, len(sessions))

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
}

func agentOrDefault(agent string) string {
	if agent == "" {
		return "default"
	}
	return agent
}
