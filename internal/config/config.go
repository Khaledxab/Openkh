package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all configuration settings for the bot.
type Config struct {
	TelegramToken string
	OpenCodeURL   string
	AllowedUsers  map[int64]bool
	AdminUsers    map[int64]bool
	WorkDir       string
	DBPath        string
	Agents        string // comma-separated "name:description" pairs
}

// LoadConfig loads configuration from environment variables with portable defaults.
func LoadConfig() *Config {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	opencodeURL := envOr("OPENCODE_URL", "http://localhost:4096")
	workDir := envOr("WORK_DIR", ".")
	dbPath := resolveDBPath()
	agents := os.Getenv("AGENTS")

	return &Config{
		TelegramToken: token,
		OpenCodeURL:   opencodeURL,
		AllowedUsers:  parseUserList(os.Getenv("ALLOWED_USERS")),
		AdminUsers:    parseUserList(os.Getenv("ADMIN_USERS")),
		WorkDir:       workDir,
		DBPath:        dbPath,
		Agents:        agents,
	}
}

// resolveDBPath determines the database file path using:
// $DB_PATH > $DATA_DIR/openkh.db > $XDG_DATA_HOME/openkh/openkh.db > ~/.local/share/openkh/openkh.db
func resolveDBPath() string {
	if p := os.Getenv("DB_PATH"); p != "" {
		return p
	}
	if p := os.Getenv("DATA_DIR"); p != "" {
		return filepath.Join(p, "openkh.db")
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "openkh.db"
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(dataHome, "openkh")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("Warning: could not create data dir %s: %v", dir, err)
		return "openkh.db"
	}
	return filepath.Join(dir, "openkh.db")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseUserList(envValue string) map[int64]bool {
	users := make(map[int64]bool)
	if envValue == "" {
		return users
	}
	for _, part := range strings.Split(envValue, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		uid, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			log.Printf("Warning: invalid user ID %q: %v", part, err)
			continue
		}
		users[uid] = true
	}
	return users
}
