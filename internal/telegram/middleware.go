package telegram

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Khaledxab/Openkh/internal/config"
	"github.com/go-telegram/bot"
)

var (
	rateLimitMap      = make(map[int64]time.Time)
	rateLimitMu       sync.RWMutex
	rateLimitDuration = 2 * time.Second
)

func checkAuth(chatID int64, cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if len(cfg.AllowedUsers) == 0 {
		return true
	}
	allowed := cfg.AllowedUsers[chatID]
	if !allowed {
		log.Printf("[AUTH BLOCKED] Unauthorized user attempt from chatID: %d", chatID)
	}
	return allowed
}

func checkRateLimit(chatID int64) bool {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()

	if lastTime, exists := rateLimitMap[chatID]; exists {
		if time.Since(lastTime) < rateLimitDuration {
			return false
		}
	}
	rateLimitMap[chatID] = time.Now()
	return true
}

func cleanupRateLimitMap() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rateLimitMu.Lock()
		threshold := time.Now().Add(-1 * time.Minute)
		for chatID, lastTime := range rateLimitMap {
			if lastTime.Before(threshold) {
				delete(rateLimitMap, chatID)
			}
		}
		rateLimitMu.Unlock()
		log.Printf("[RATE LIMIT] Cleanup completed. Active entries: %d", len(rateLimitMap))
	}
}

func (b *Bot) requireAuth(chatID int64, tgBot *bot.Bot, ctx context.Context) bool {
	if b.Config == nil {
		return true
	}
	if !checkAuth(chatID, b.Config) {
		tgBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Unauthorized. You are not allowed to use this bot.",
		})
		return false
	}
	return true
}

func (b *Bot) isAdmin(chatID int64) bool {
	if b.Config == nil || len(b.Config.AdminUsers) == 0 {
		return true
	}
	return b.Config.AdminUsers[chatID]
}
