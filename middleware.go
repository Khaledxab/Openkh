package main

import (
	"log"
	"sync"
	"time"
)

var (
	rateLimitMap       = make(map[int64]time.Time)
	rateLimitMu        sync.RWMutex
	rateLimitDuration  = 2 * time.Second
)

// checkAuth validates user access against ALLOWED_USERS.
// Returns true if allowed, false otherwise. Logs blocked attempts.
// If ALLOWED_USERS is empty, allows all users (no restrictions).
func checkAuth(chatID int64, cfg *Config) bool {
	if cfg == nil {
		log.Printf("[AUTH] Config is nil, blocking chatID: %d", chatID)
		return false
	}

	if cfg.ALLOWED_USERS == nil || len(cfg.ALLOWED_USERS) == 0 {
		return true
	}

	allowed := cfg.ALLOWED_USERS[chatID]
	if !allowed {
		log.Printf("[AUTH BLOCKED] Unauthorized user attempt from chatID: %d", chatID)
	}
	return allowed
}

// checkRateLimit enforces per-user rate limiting.
// Returns true if allowed, false if must wait.
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

// cleanupRateLimitMap periodically removes stale entries from rateLimitMap.
// Runs every 5 minutes, deletes entries older than 1 minute.
func cleanupRateLimitMap() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rateLimitMu.Lock()
		now := time.Now()
		threshold := now.Add(-1 * time.Minute)

		for chatID, lastTime := range rateLimitMap {
			if lastTime.Before(threshold) {
				delete(rateLimitMap, chatID)
			}
		}
		rateLimitMu.Unlock()

		log.Printf("[RATE LIMIT] Cleanup completed. Active entries: %d", len(rateLimitMap))
	}
}
