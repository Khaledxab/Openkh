package telegram

// shortID safely truncates an ID to 8 characters + "..." for display.
// Returns the full ID if it's shorter than 8 characters.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8] + "..."
}

// currentSessionID returns the OpenCode session ID for a chat, or "".
func (b *Bot) currentSessionID(chatID int64) string {
	if b.DB == nil {
		return ""
	}
	sess, err := b.DB.GetSession(chatID)
	if err != nil {
		return ""
	}
	return sess.SessionID
}

// currentAgent returns the agent for a chat, defaulting to "sisyphus".
func (b *Bot) currentAgent(chatID int64) string {
	if b.DB == nil {
		return "sisyphus"
	}
	sess, err := b.DB.GetSession(chatID)
	if err != nil || sess.Agent == "" {
		return "sisyphus"
	}
	return sess.Agent
}
