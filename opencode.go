package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OCSession represents an OpenCode session from the API
type OCSession struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	ProjectID string `json:"projectID"`
	Directory string `json:"directory"`
	Time      struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
}

// Message represents a chat message
type Message struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version"`
}

// SuccessResponse represents a generic success response
type SuccessResponse struct {
	Success bool `json:"success"`
}

// OpenCodeClient wraps HTTP client for OpenCode API
type OpenCodeClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewOpenCodeClient creates a new OpenCode client with the given base URL
func NewOpenCodeClient(baseURL string) *OpenCodeClient {
	return &OpenCodeClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Health checks the health of the OpenCode server
func (c *OpenCodeClient) Health(ctx context.Context) error {
	url := c.baseURL + "/global/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to check health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	var health HealthResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read health response: %w", err)
	}

	if err := json.Unmarshal(body, &health); err != nil {
		return fmt.Errorf("failed to parse health response: %w", err)
	}

	if !health.Healthy {
		return fmt.Errorf("server is not healthy")
	}

	return nil
}

// CreateOCSession creates a new OpenCode session
func (c *OpenCodeClient) CreateOCSession(ctx context.Context, title string) (OCSession, error) {
	url := c.baseURL + "/session"

	body := map[string]string{"title": title}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return OCSession{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return OCSession{}, fmt.Errorf("failed to create session request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return OCSession{}, fmt.Errorf("failed to create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return OCSession{}, fmt.Errorf("create session failed with status: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return OCSession{}, fmt.Errorf("failed to read create session response: %w", err)
	}

	var session OCSession
	if err := json.Unmarshal(respBody, &session); err != nil {
		return OCSession{}, fmt.Errorf("failed to parse session response: %w", err)
	}

	return session, nil
}

// ListOCSessions returns all OpenCode sessions
func (c *OpenCodeClient) ListOCSessions(ctx context.Context) ([]OCSession, error) {
	url := c.baseURL + "/session"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list sessions request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list sessions failed with status: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read list sessions response: %w", err)
	}

	var sessions []OCSession
	if err := json.Unmarshal(respBody, &sessions); err != nil {
		return nil, fmt.Errorf("failed to parse sessions response: %w", err)
	}

	return sessions, nil
}

// GetOCSession returns a specific session by ID
func (c *OpenCodeClient) GetOCSession(ctx context.Context, id string) (OCSession, error) {
	url := c.baseURL + "/session/" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return OCSession{}, fmt.Errorf("failed to create get session request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return OCSession{}, fmt.Errorf("failed to get session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return OCSession{}, fmt.Errorf("get session failed with status: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return OCSession{}, fmt.Errorf("failed to read get session response: %w", err)
	}

	var session OCSession
	if err := json.Unmarshal(respBody, &session); err != nil {
		return OCSession{}, fmt.Errorf("failed to parse session response: %w", err)
	}

	return session, nil
}

// DeleteOCSession deletes a session by ID
func (c *OpenCodeClient) DeleteOCSession(ctx context.Context, id string) error {
	url := c.baseURL + "/session/" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete session request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete session failed with status: %d", resp.StatusCode)
	}

	return nil
}

// GetMessages returns all messages for a session
func (c *OpenCodeClient) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	url := c.baseURL + "/session/" + sessionID + "/message"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get messages request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get messages failed with status: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read get messages response: %w", err)
	}

	var messages []Message
	if err := json.Unmarshal(respBody, &messages); err != nil {
		return nil, fmt.Errorf("failed to parse messages response: %w", err)
	}

	return messages, nil
}

// PromptAsync sends a prompt to a session asynchronously
func (c *OpenCodeClient) PromptAsync(ctx context.Context, sessionID, text string) error {
	url := c.baseURL + "/session/" + sessionID + "/prompt_async"

	// API expects: {"parts": [{"type": "text", "text": "message"}]}
	body := map[string][]map[string]string{
		"parts": {
			{"type": "text", "text": text},
		},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create prompt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send prompt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("prompt failed with status: %d", resp.StatusCode)
	}

	// 204 No Content means async prompt accepted, no body to parse
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read prompt response: %w", err)
	}

	var success SuccessResponse
	if err := json.Unmarshal(respBody, &success); err != nil {
		return fmt.Errorf("failed to parse prompt response: %w", err)
	}

	if !success.Success {
		return fmt.Errorf("prompt was not successful")
	}

	return nil
}

// Abort aborts the current operation in a session
func (c *OpenCodeClient) Abort(ctx context.Context, sessionID string) error {
	url := c.baseURL + "/session/" + sessionID + "/abort"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create abort request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to abort: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("abort failed with status: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read abort response: %w", err)
	}

	var success SuccessResponse
	if err := json.Unmarshal(respBody, &success); err != nil {
		return fmt.Errorf("failed to parse abort response: %w", err)
	}

	if !success.Success {
		return fmt.Errorf("abort was not successful")
	}

	return nil
}

// GetDiff returns the diff for a session
func (c *OpenCodeClient) GetDiff(ctx context.Context, sessionID string) (string, error) {
	url := c.baseURL + "/session/" + sessionID + "/diff"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create get diff request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get diff failed with status: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read get diff response: %w", err)
	}

	return string(respBody), nil
}
