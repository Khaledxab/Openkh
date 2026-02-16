package opencode

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
)

// MessageSender abstracts sending/editing messages so StreamManager
// doesn't depend on any specific Telegram library.
type MessageSender interface {
	SendText(chatID int64, text string) (messageID int, err error)
	EditText(chatID int64, messageID int, text string) error
}

// StreamManager handles SSE streaming from OpenCode and dispatches
// updates through a MessageSender.
type StreamManager struct {
	baseURL        string
	httpClient     *http.Client
	sender         MessageSender
	sessionToChat  map[string]int64
	chatToMsgID    map[int64]int
	chatToText     map[int64]string
	chatToStatus   map[int64]string
	reasoningParts map[string]bool
	textPartIDs    map[int64]string
	lastEdit       map[int64]time.Time
	editThrottle   time.Duration
	mu             sync.RWMutex
}

// NewStreamManager creates a StreamManager backed by the given MessageSender.
func NewStreamManager(baseURL string, sender MessageSender) *StreamManager {
	return &StreamManager{
		baseURL:        baseURL,
		httpClient:     &http.Client{Timeout: 0},
		sender:         sender,
		sessionToChat:  make(map[string]int64),
		chatToMsgID:    make(map[int64]int),
		chatToText:     make(map[int64]string),
		chatToStatus:   make(map[int64]string),
		reasoningParts: make(map[string]bool),
		textPartIDs:    make(map[int64]string),
		lastEdit:       make(map[int64]time.Time),
		editThrottle:   1 * time.Second,
	}
}

// Start connects to the SSE endpoint and processes events. It reconnects on error.
func (sm *StreamManager) Start(ctx context.Context) error {
	url := sm.baseURL + "/event"
	log.Printf("[StreamManager] Starting SSE connection to %s", url)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := sm.connectAndRead(ctx, url); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("[StreamManager] Connection error: %v, retrying in 2s...", err)
			time.Sleep(2 * time.Second)
		}
	}
}

func (sm *StreamManager) connectAndRead(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := sm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	log.Println("[StreamManager] Connected to SSE stream")

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

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
				return fmt.Errorf("scanner: %w", err)
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
	var event SSEEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		log.Printf("[StreamManager] Failed to parse event: %v", err)
		return
	}
	sm.handleEvent(event)
}

// RegisterSession maps an OpenCode session ID to a Telegram chat + message.
func (sm *StreamManager) RegisterSession(sessionID string, chatID int64, messageID int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sessionToChat[sessionID] = chatID
	sm.chatToMsgID[chatID] = messageID
	sm.chatToText[chatID] = ""
	sm.chatToStatus[chatID] = ""
	sm.textPartIDs[chatID] = ""
	sm.lastEdit[chatID] = time.Time{}
	log.Printf("[StreamManager] Registered session %s -> chat %d, message %d", sessionID, chatID, messageID)
}

// UnregisterSession removes a session mapping.
func (sm *StreamManager) UnregisterSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if chatID, ok := sm.sessionToChat[sessionID]; ok {
		delete(sm.sessionToChat, sessionID)
		delete(sm.chatToMsgID, chatID)
		delete(sm.chatToText, chatID)
		delete(sm.chatToStatus, chatID)
		delete(sm.textPartIDs, chatID)
		delete(sm.lastEdit, chatID)
	}
}

// GetActiveSessionCount returns the number of tracked sessions.
func (sm *StreamManager) GetActiveSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessionToChat)
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
		// handled by message.updated finish detection
	case "server.connected", "server.heartbeat", "session.created", "session.updated", "session.status", "session.diff":
		// ignore
	default:
		log.Printf("[StreamManager] Unhandled event: %s", event.Type)
	}
}

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
		sm.mu.Lock()
		sm.reasoningParts[props.Part.ID] = true
		if props.Part.Text == "" {
			sm.chatToStatus[chatID] = "Thinking..."
		} else {
			sm.chatToStatus[chatID] = ""
		}
		sm.mu.Unlock()
		sm.editMessage(chatID)
	case "step-start":
		sm.mu.Lock()
		sm.chatToStatus[chatID] = "Processing..."
		sm.mu.Unlock()
		sm.editMessage(chatID)
	case "step-finish":
		sm.mu.Lock()
		sm.chatToStatus[chatID] = ""
		sm.mu.Unlock()
	case "tool-invocation", "tool-call":
		sm.mu.Lock()
		sm.chatToStatus[chatID] = "Running tool..."
		sm.mu.Unlock()
		sm.editMessage(chatID)
	case "tool-result":
		sm.mu.Lock()
		sm.chatToStatus[chatID] = ""
		sm.mu.Unlock()
	}
}

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
	if !ok || isReasoning {
		return
	}

	sm.mu.Lock()
	sm.chatToText[chatID] += props.Delta
	sm.chatToStatus[chatID] = ""
	sm.mu.Unlock()

	sm.editMessage(chatID)
}

func (sm *StreamManager) handleMessageUpdated(raw json.RawMessage) {
	var props MessageProperties
	if err := json.Unmarshal(raw, &props); err != nil {
		return
	}
	sessionID := props.Info.SessionID
	if sessionID == "" || props.Info.Role != "assistant" {
		return
	}
	if props.Info.Finish != "" {
		sm.mu.RLock()
		chatID, ok := sm.sessionToChat[sessionID]
		sm.mu.RUnlock()
		if ok {
			sm.markComplete(chatID, sessionID)
		}
	}
}

func (sm *StreamManager) editMessage(chatID int64) {
	if !sm.canEdit(chatID) {
		return
	}

	sm.mu.RLock()
	messageID, hasMsg := sm.chatToMsgID[chatID]
	text := sm.chatToText[chatID]
	status := sm.chatToStatus[chatID]
	sm.mu.RUnlock()

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
	if len(display) > 4000 {
		display = display[:4000] + "\n\n... (truncated)"
	}

	if !hasMsg {
		msgID, err := sm.sender.SendText(chatID, display)
		if err != nil {
			log.Printf("[StreamManager] Failed to send: %v", err)
			return
		}
		sm.mu.Lock()
		sm.chatToMsgID[chatID] = msgID
		sm.mu.Unlock()
	} else {
		if err := sm.sender.EditText(chatID, messageID, display); err != nil {
			if !strings.Contains(err.Error(), "message is not modified") {
				log.Printf("[StreamManager] Failed to edit: %v", err)
			}
		}
	}

	sm.mu.Lock()
	sm.lastEdit[chatID] = time.Now()
	sm.mu.Unlock()
}

func (sm *StreamManager) markComplete(chatID int64, sessionID string) {
	sm.mu.RLock()
	messageID, hasMsg := sm.chatToMsgID[chatID]
	text := sm.chatToText[chatID]
	sm.mu.RUnlock()

	if !hasMsg {
		return
	}
	if text == "" {
		text = "Completed"
	}
	if len(text) > 4000 {
		text = text[:4000] + "\n\n... (truncated)"
	}

	if err := sm.sender.EditText(chatID, messageID, text); err != nil {
		if !strings.Contains(err.Error(), "message is not modified") {
			log.Printf("[StreamManager] Failed to mark complete: %v", err)
		}
	}
	log.Printf("[StreamManager] Complete for chat %d", chatID)

	sm.mu.Lock()
	delete(sm.chatToMsgID, chatID)
	delete(sm.chatToText, chatID)
	delete(sm.chatToStatus, chatID)
	delete(sm.textPartIDs, chatID)
	delete(sm.lastEdit, chatID)
	for k := range sm.reasoningParts {
		delete(sm.reasoningParts, k)
	}
	sm.mu.Unlock()
}

func (sm *StreamManager) canEdit(chatID int64) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	last, ok := sm.lastEdit[chatID]
	if !ok {
		return true
	}
	return time.Since(last) >= sm.editThrottle
}
