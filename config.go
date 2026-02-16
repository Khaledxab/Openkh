package main

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration settings for the bot
type Config struct {
	TELEGRAM_BOT_TOKEN string         // Telegram bot token (required)
	OPENCODE_URL     string         // OpenCode server URL (default: http://localhost:4096)
	ALLOWED_USERS    map[int64]bool // Map of allowed user IDs
	ADMIN_USERS      map[int64]bool // Map of admin user IDs
	WORK_DIR         string         // Working directory for bot operations (default: /home/khale)
}

// Default values
const (
	DefaultOpenCodeURL = "http://localhost:4096"
	DefaultWorkDir     = "/home/khale"
)

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	// Load required TELEGRAM_BOT_TOKEN
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required but not set")
	}

	// Load OPENCODE_URL with default
	opencodeURL := os.Getenv("OPENCODE_URL")
	if opencodeURL == "" {
		opencodeURL = DefaultOpenCodeURL
	}

	// Load WORK_DIR with default
	workDir := os.Getenv("WORK_DIR")
	if workDir == "" {
		workDir = DefaultWorkDir
	}

	// Parse ALLOWED_USERS from comma-separated string
	allowedUsers := parseUserList(os.Getenv("ALLOWED_USERS"))

	// Parse ADMIN_USERS from comma-separated string
	adminUsers := parseUserList(os.Getenv("ADMIN_USERS"))

	return &Config{
		TELEGRAM_BOT_TOKEN: token,
		OPENCODE_URL:     opencodeURL,
		ALLOWED_USERS:    allowedUsers,
		ADMIN_USERS:      adminUsers,
		WORK_DIR:         workDir,
	}
}

// parseUserList parses a comma-separated string of user IDs into a map[int64]bool
// Expected format: "6144852203,1234567890"
func parseUserList(envValue string) map[int64]bool {
	users := make(map[int64]bool)

	if envValue == "" {
		return users
	}

	// Split by comma
	parts := strings.Split(envValue, ",")

	for _, part := range parts {
		// Trim whitespace
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Parse to int64
		userID, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			log.Printf("Warning: invalid user ID '%s': %v", part, err)
			continue
		}

		users[userID] = true
	}

	return users
}
