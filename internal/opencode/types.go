package opencode

import "encoding/json"

// OCSession represents an OpenCode session from the API.
type OCSession struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	ProjectID string `json:"projectID"`
	Directory string `json:"directory"`
	Version   string `json:"version"`
	Summary   struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
		Files     int `json:"files"`
	} `json:"summary"`
	Time struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
}

// APIMessage represents a message from the OpenCode API.
type APIMessage struct {
	Info struct {
		ID        string `json:"id"`
		SessionID string `json:"sessionID"`
		Role      string `json:"role"`
		Tokens    struct {
			Total  int `json:"total"`
			Input  int `json:"input"`
			Output int `json:"output"`
		} `json:"tokens"`
		Cost   float64 `json:"cost"`
		Finish string  `json:"finish"`
	} `json:"info"`
	Parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"parts"`
}

// Message is a simplified message for display.
type Message struct {
	ID      string
	Role    string
	Content string
	Tokens  int
	Cost    float64
}

// SSEEvent represents a Server-Sent Events message.
type SSEEvent struct {
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties"`
}

// PartProperties represents a message.part.updated event.
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

// DeltaProperties represents a message.part.delta event.
type DeltaProperties struct {
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	PartID    string `json:"partID"`
	Field     string `json:"field"`
	Delta     string `json:"delta"`
}

// MessageProperties represents a message.updated event.
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

// SessionStatusProperties represents session.status / session.idle events.
type SessionStatusProperties struct {
	SessionID string `json:"sessionID"`
	Status    struct {
		Type string `json:"type"`
	} `json:"status"`
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version"`
}

// SuccessResponse represents a generic success response.
type SuccessResponse struct {
	Success bool `json:"success"`
}
