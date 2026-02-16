package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps the HTTP client for the OpenCode API.
type Client struct {
	BaseURL    string
	httpClient *http.Client
}

// NewClient creates a new OpenCode client.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Health checks the health of the OpenCode server.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/global/health", nil)
	if err != nil {
		return fmt.Errorf("create health request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read health response: %w", err)
	}
	var h HealthResponse
	if err := json.Unmarshal(body, &h); err != nil {
		return fmt.Errorf("parse health response: %w", err)
	}
	if !h.Healthy {
		return fmt.Errorf("server is not healthy")
	}
	return nil
}

// GetProviders fetches available model providers from the OpenCode server.
func (c *Client) GetProviders(ctx context.Context) (ProviderResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/provider", nil)
	if err != nil {
		return ProviderResponse{}, fmt.Errorf("create providers request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ProviderResponse{}, fmt.Errorf("get providers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ProviderResponse{}, fmt.Errorf("get providers status: %d", resp.StatusCode)
	}
	return decodeJSON[ProviderResponse](resp.Body)
}

// CreateOCSession creates a new OpenCode session.
func (c *Client) CreateOCSession(ctx context.Context, title string) (OCSession, error) {
	body, _ := json.Marshal(map[string]string{"title": title})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/session", bytes.NewReader(body))
	if err != nil {
		return OCSession{}, fmt.Errorf("create session request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return OCSession{}, fmt.Errorf("create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return OCSession{}, fmt.Errorf("create session status: %d", resp.StatusCode)
	}
	return decodeJSON[OCSession](resp.Body)
}

// ListOCSessions returns all OpenCode sessions.
func (c *Client) ListOCSessions(ctx context.Context) ([]OCSession, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/session", nil)
	if err != nil {
		return nil, fmt.Errorf("list sessions request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list sessions status: %d", resp.StatusCode)
	}
	return decodeJSON[[]OCSession](resp.Body)
}

// GetOCSession returns a specific session by ID.
func (c *Client) GetOCSession(ctx context.Context, id string) (OCSession, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/session/"+id, nil)
	if err != nil {
		return OCSession{}, fmt.Errorf("get session request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return OCSession{}, fmt.Errorf("get session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return OCSession{}, fmt.Errorf("get session status: %d", resp.StatusCode)
	}
	return decodeJSON[OCSession](resp.Body)
}

// DeleteOCSession deletes a session by ID.
func (c *Client) DeleteOCSession(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.BaseURL+"/session/"+id, nil)
	if err != nil {
		return fmt.Errorf("delete session request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete session status: %d", resp.StatusCode)
	}
	return nil
}

// RenameOCSession updates the title of an existing session.
func (c *Client) RenameOCSession(ctx context.Context, id, newTitle string) (OCSession, error) {
	body, _ := json.Marshal(map[string]string{"title": newTitle})
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.BaseURL+"/session/"+id, bytes.NewReader(body))
	if err != nil {
		return OCSession{}, fmt.Errorf("rename session request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return OCSession{}, fmt.Errorf("rename session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return OCSession{}, fmt.Errorf("rename session status: %d", resp.StatusCode)
	}
	return decodeJSON[OCSession](resp.Body)
}

// GetMessages returns all messages for a session.
func (c *Client) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/session/"+sessionID+"/message", nil)
	if err != nil {
		return nil, fmt.Errorf("get messages request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get messages status: %d", resp.StatusCode)
	}

	apiMsgs, err := decodeJSON[[]APIMessage](resp.Body)
	if err != nil {
		return nil, err
	}

	var messages []Message
	for _, am := range apiMsgs {
		var content string
		for _, p := range am.Parts {
			if p.Type == "text" && p.Text != "" {
				if content != "" {
					content += "\n"
				}
				content += p.Text
			}
		}
		messages = append(messages, Message{
			ID:      am.Info.ID,
			Role:    am.Info.Role,
			Content: content,
			Tokens:  am.Info.Tokens.Total,
			Cost:    am.Info.Cost,
		})
	}
	return messages, nil
}

// PromptAsync sends a prompt to a session asynchronously.
func (c *Client) PromptAsync(ctx context.Context, sessionID, text, agent, providerID, modelID string) error {
	payload := map[string]interface{}{
		"parts": []map[string]string{
			{"type": "text", "text": text},
		},
	}
	if agent != "" {
		payload["agent"] = agent
	}
	if providerID != "" && modelID != "" {
		payload["model"] = map[string]string{
			"providerID": providerID,
			"modelID":   modelID,
		}
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/session/"+sessionID+"/prompt_async", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create prompt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send prompt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("prompt status: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read prompt response: %w", err)
	}
	var s SuccessResponse
	if err := json.Unmarshal(respBody, &s); err != nil {
		return fmt.Errorf("parse prompt response: %w", err)
	}
	if !s.Success {
		return fmt.Errorf("prompt was not successful")
	}
	return nil
}

// Abort aborts the current operation in a session.
func (c *Client) Abort(ctx context.Context, sessionID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/session/"+sessionID+"/abort", nil)
	if err != nil {
		return fmt.Errorf("create abort request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("abort: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("abort status: %d", resp.StatusCode)
	}
	return nil
}

// GetDiff returns the diff for a session.
func (c *Client) GetDiff(ctx context.Context, sessionID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/session/"+sessionID+"/diff", nil)
	if err != nil {
		return "", fmt.Errorf("get diff request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get diff: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get diff status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read diff response: %w", err)
	}
	return string(body), nil
}

func decodeJSON[T any](r io.Reader) (T, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("read response: %w", err)
	}
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		return v, fmt.Errorf("parse response: %w", err)
	}
	return v, nil
}
