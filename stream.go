package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
)

// SSEEvent represents a Server-Sent Events message
type SSEEvent struct {
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties"`
}

// PartProperties represents the "part" field inside message.part.updated / message.part.delta
type PartProperties struct {
	Part struct {
		ID        string `json:"id"`
		SessionID string `json:"sessionID"`
		MessageID string `json:"messageID"`
		Type      string `json:"type"`
		Text      string `json:"text"`
		Time      struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		} `json:"time"`
	} `json:"part"`
}

// DeltaProperties represents a message.part.delta event
type DeltaProperties struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	PartID    string `json:"partID"`
	Field     string `json:"field"`
	Delta     string `json:"delta"`
}

// MessageProperties represents a message.updated event
type MessageProperties struct {
	Info struct {
		ID        string `json:"id"`
		SessionID string `json:"sessionID"`
		Role      string `json:"role"`
		Finish    string `json:"finish"`
		Time      struct {
			Created   int64 `json:"created"`
			Completed int64 `json:"completed"`
		} `json:"time"`
	} `json:"info"`
}

// SessionStatusProperties represents session.status / session.idle events
type SessionStatusProperties struct {
	SessionID string `json:"sessionID"`
	Status    struct {
		Type string `json:"type"`
	} `json:"status"`
}

type StreamManager struct {
	baseURL         string
	httpClient      *http.Client
	sessionToChat   map[string]int64
	chatToMessageID map[int64]int
	chatToText      map[int64]string // accumulated response text
	chatToStatus    map[int64]string // current status line (thinking, tool, etc.)
	reasoningParts  map[string]bool  // partIDs that are reasoning (not text)
	textPartIDs     map[int64]string // chatID -> active text partID
	mu              sync.RWMutex
	bot             *bot.Bot
	lastEdit        map[int64]time.Time
	editThrottle    time.Duration
}

func NewStreamManager(baseURL string, botInstance *bot.Bot) *StreamManager {
	return &StreamManager{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 0,
		},
		sessionToChat:   make(map[string]int64),
		chatToMessageID: make(map[int64]int),
		chatToText:      make(map[int64]string),
		chatToStatus:    make(map[int64]string),
		reasoningParts:  make(map[string]bool),
		textPartIDs:     make(map[int64]string),
		bot:             botInstance,
		lastEdit:        make(map[int64]time.Time),
		editThrottle:    1 * time.Second,
	}
}

func (sm *StreamManager) Start(ctx context.Context) error {
	url := sm.baseURL + "/event"
	log.Printf("[StreamManager] Starting SSE connection to %s", url)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := sm.connectAndRead(ctx, url)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("[StreamManager] Connection error: %v, retrying in 2s...", err)
			time.Sleep(2 * time.Second)
			continue
		}
	}
}

func (sm *StreamManager) connectAndRead(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := sm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Println("[StreamManager] Connected to SSE stream")

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var eventData string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("scanner error: %w", err)
			}
			return fmt.Errorf("SSE stream closed unexpectedly")
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "data: ") {
			eventData = strings.TrimPrefix(line, "data: ")
		} else if line == "" && eventData != "" {
			sm.processEventData(eventData)
			eventData = ""
		}
	}
}

func (sm *StreamManager) processEventData(data string) {
	if data == "" {
		return
	}

	var event SSEEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		log.Printf("[StreamManager] Failed to parse event: %v", err)
		return
	}

	sm.handleEvent(event)
}

func (sm *StreamManager) RegisterSession(sessionID string, chatID int64, messageID int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sessionToChat[sessionID] = chatID
	sm.chatToMessageID[chatID] = messageID
	sm.chatToText[chatID] = ""
	sm.chatToStatus[chatID] = ""
	sm.textPartIDs[chatID] = ""
	sm.lastEdit[chatID] = time.Time{}

	log.Printf("[StreamManager] Registered session %s -> chat %d, message %d", sessionID, chatID, messageID)
}

func (sm *StreamManager) UnregisterSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if chatID, ok := sm.sessionToChat[sessionID]; ok {
		delete(sm.sessionToChat, sessionID)
		delete(sm.chatToMessageID, chatID)
		delete(sm.chatToText, chatID)
		delete(sm.chatToStatus, chatID)
		delete(sm.textPartIDs, chatID)
		delete(sm.lastEdit, chatID)
	}
}

func (sm *StreamManager) handleEvent(event SSEEvent) {
	switch event.Type {
	case "message.part.updated":
		sm.handlePartUpdated(event.Properties)
	case "message.part.delta":
		sm.handlePartDelta(event.Properties)
	case "message.updated":
		sm.handleMessageUpdated(event.Properties)
	case "session.idle":
		// handled by message.updated finish detection instead
	case "server.connected", "server.heartbeat", "session.created", "session.updated", "session.status", "session.diff":
		// ignore silently
	default:
		log.Printf("[StreamManager] Unhandled event: %s", event.Type)
	}
}

// handlePartUpdated handles message.part.updated events (full part snapshots)
func (sm *StreamManager) handlePartUpdated(raw json.RawMessage) {
	var props PartProperties
	if err := json.Unmarshal(raw, &props); err != nil {
		log.Printf("[StreamManager] Failed to parse part.updated: %v", err)
		return
	}

	sessionID := props.Part.SessionID
	if sessionID == "" {
		return
	}

	sm.mu.RLock()
	chatID, ok := sm.sessionToChat[sessionID]
	sm.mu.RUnlock()
	if !ok {
		return
	}

	switch props.Part.Type {
	case "text":
		// Register this as a text part
		sm.mu.Lock()
		sm.textPartIDs[chatID] = props.Part.ID
		if props.Part.Text != "" {
			sm.chatToText[chatID] = props.Part.Text
		}
		sm.chatToStatus[chatID] = ""
		sm.mu.Unlock()
		if props.Part.Text != "" {
			sm.editMessage(chatID)
		}
	case "reasoning":
		// Mark this partID as reasoning so deltas are ignored
		sm.mu.Lock()
		sm.reasoningParts[props.Part.ID] = true
		if props.Part.Text == "" {
			sm.chatToStatus[chatID] = "💭 Thinking..."
		} else {
			// reasoning complete, clear status
			sm.chatToStatus[chatID] = ""
		}
		sm.mu.Unlock()
		sm.editMessage(chatID)
	case "step-start":
		sm.mu.Lock()
		sm.chatToStatus[chatID] = "⚙️ Processing..."
		sm.mu.Unlock()
		sm.editMessage(chatID)
	case "step-finish":
		sm.mu.Lock()
		sm.chatToStatus[chatID] = ""
		sm.mu.Unlock()
	case "tool-invocation", "tool-call":
		sm.mu.Lock()
		sm.chatToStatus[chatID] = "🔧 Running tool..."
		sm.mu.Unlock()
		sm.editMessage(chatID)
	case "tool-result":
		sm.mu.Lock()
		sm.chatToStatus[chatID] = ""
		sm.mu.Unlock()
	}
}

// handlePartDelta handles message.part.delta events (incremental text deltas)
func (sm *StreamManager) handlePartDelta(raw json.RawMessage) {
	var props DeltaProperties
	if err := json.Unmarshal(raw, &props); err != nil {
		log.Printf("[StreamManager] Failed to parse part.delta: %v", err)
		return
	}

	if props.SessionID == "" || props.Field != "text" {
		return
	}

	sm.mu.RLock()
	chatID, ok := sm.sessionToChat[props.SessionID]
	isReasoning := sm.reasoningParts[props.PartID]
	sm.mu.RUnlock()
	if !ok {
		return
	}

	// Skip reasoning deltas — only accumulate text part deltas
	if isReasoning {
		return
	}

	sm.mu.Lock()
	sm.chatToText[chatID] += props.Delta
	sm.chatToStatus[chatID] = ""
	sm.mu.Unlock()

	sm.editMessage(chatID)
}

// handleMessageUpdated handles message.updated events (check for completion)
func (sm *StreamManager) handleMessageUpdated(raw json.RawMessage) {
	var props MessageProperties
	if err := json.Unmarshal(raw, &props); err != nil {
		return
	}

	sessionID := props.Info.SessionID
	if sessionID == "" || props.Info.Role != "assistant" {
		return
	}

	// Check if the message is completed (has a finish reason)
	if props.Info.Finish != "" {
		sm.mu.RLock()
		chatID, ok := sm.sessionToChat[sessionID]
		sm.mu.RUnlock()
		if ok {
			sm.markComplete(chatID, sessionID)
		}
	}
}

// handleSessionIdle handles session.idle events
func (sm *StreamManager) handleSessionIdle(raw json.RawMessage) {
	var props SessionStatusProperties
	if err := json.Unmarshal(raw, &props); err != nil {
		return
	}

	if props.SessionID == "" {
		return
	}

	sm.mu.RLock()
	chatID, ok := sm.sessionToChat[props.SessionID]
	sm.mu.RUnlock()
	if ok {
		sm.markComplete(chatID, props.SessionID)
	}
}

// editMessage composes the full display text and edits the Telegram message
func (sm *StreamManager) editMessage(chatID int64) {
	if !sm.canEdit(chatID) {
		return
	}

	sm.mu.RLock()
	messageID, hasMsg := sm.chatToMessageID[chatID]
	text := sm.chatToText[chatID]
	status := sm.chatToStatus[chatID]
	sm.mu.RUnlock()

	// Compose display text
	display := text
	if status != "" {
		if display != "" {
			display = display + "\n\n" + status
		} else {
			display = status
		}
	}

	if display == "" {
		return
	}

	// Truncate for Telegram limit
	if len(display) > 4000 {
		display = display[:4000] + "\n\n_... (truncated)_"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if !hasMsg {
		msg, err := sm.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   display,
		})
		if err != nil {
			log.Printf("[StreamManager] Failed to send: %v", err)
			return
		}
		sm.mu.Lock()
		sm.chatToMessageID[chatID] = msg.ID
		sm.mu.Unlock()
	} else {
		_, err := sm.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      display,
		})
		if err != nil {
			// "message is not modified" is normal when content hasn't changed
			if !strings.Contains(err.Error(), "message is not modified") {
				log.Printf("[StreamManager] Failed to edit: %v", err)
			}
		}
	}

	sm.updateLastEdit(chatID)
}

func (sm *StreamManager) markComplete(chatID int64, sessionID string) {
	sm.mu.RLock()
	messageID, hasMsg := sm.chatToMessageID[chatID]
	text := sm.chatToText[chatID]
	sm.mu.RUnlock()

	if !hasMsg {
		return
	}

	// Final edit — strip any lingering status line, show clean response
	if text == "" {
		text = "✅ Completed"
	}

	if len(text) > 4000 {
		text = text[:4000] + "\n\n_... (truncated)_"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := sm.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
	})
	if err != nil {
		if !strings.Contains(err.Error(), "message is not modified") {
			log.Printf("[StreamManager] Failed to mark complete: %v", err)
		}
	}

	log.Printf("[StreamManager] Complete for chat %d", chatID)

	// Cleanup
	sm.mu.Lock()
	delete(sm.chatToMessageID, chatID)
	delete(sm.chatToText, chatID)
	delete(sm.chatToStatus, chatID)
	delete(sm.textPartIDs, chatID)
	delete(sm.lastEdit, chatID)
	// Clean up reasoning parts for this session
	for k := range sm.reasoningParts {
		delete(sm.reasoningParts, k)
	}
	// Keep sessionToChat so future prompts can reuse
	sm.mu.Unlock()
}

func (sm *StreamManager) canEdit(chatID int64) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	lastEdit, ok := sm.lastEdit[chatID]
	if !ok {
		return true
	}

	return time.Since(lastEdit) >= sm.editThrottle
}

func (sm *StreamManager) updateLastEdit(chatID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastEdit[chatID] = time.Now()
}

func (sm *StreamManager) GetSessionChatID(sessionID string) (int64, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	chatID, ok := sm.sessionToChat[sessionID]
	return chatID, ok
}

func (sm *StreamManager) GetActiveSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessionToChat)
}
