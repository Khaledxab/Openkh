package store

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Session represents a user's session mapping in the database.
type Session struct {
	ChatID       int64
	SessionID    string
	Title        string
	Agent        string
	ModelProvider string
	ModelID       string
	MessageCount int
	CreatedAt    time.Time
	LastUsed     time.Time
}

// DB wraps a SQLite database for session management.
type DB struct {
	*sql.DB
}

// New opens the database at dbPath and initializes the schema.
func New(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	d := &DB{db}
	if err := d.init(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

func (db *DB) init() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS user_sessions (
			chat_id       INTEGER PRIMARY KEY,
			session_id    TEXT NOT NULL,
			title         TEXT,
			agent         TEXT DEFAULT '',
			model_provider TEXT DEFAULT '',
			model_id       TEXT DEFAULT '',
			message_count INTEGER DEFAULT 0,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_used     DATETIME DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return err
	}
	// Add agent column if migrating from old schema
	_, _ = db.Exec(`ALTER TABLE user_sessions ADD COLUMN agent TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE user_sessions ADD COLUMN model_provider TEXT DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE user_sessions ADD COLUMN model_id TEXT DEFAULT ''`)
	log.Println("Database initialized successfully")
	return nil
}

// GetSession retrieves the session for a chat ID.
func (db *DB) GetSession(chatID int64) (Session, error) {
	var s Session
	var agent sql.NullString
	var modelProvider sql.NullString
	var modelID sql.NullString
	err := db.QueryRow(`
		SELECT chat_id, session_id, title, agent, model_provider, model_id, message_count, created_at, last_used
		FROM user_sessions WHERE chat_id = ?`, chatID,
	).Scan(&s.ChatID, &s.SessionID, &s.Title, &agent, &modelProvider, &modelID, &s.MessageCount, &s.CreatedAt, &s.LastUsed)
	if err != nil {
		return Session{}, err
	}
	s.Agent = agent.String
	s.ModelProvider = modelProvider.String
	s.ModelID = modelID.String
	return s, nil
}

// SetSession upserts a session mapping.
func (db *DB) SetSession(s Session) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO user_sessions
			(chat_id, session_id, title, agent, model_provider, model_id, message_count, created_at, last_used)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ChatID, s.SessionID, s.Title, s.Agent, s.ModelProvider, s.ModelID, s.MessageCount, s.CreatedAt, s.LastUsed)
	return err
}

// DeleteSession removes a session by chat ID.
func (db *DB) DeleteSession(chatID int64) error {
	_, err := db.Exec(`DELETE FROM user_sessions WHERE chat_id = ?`, chatID)
	return err
}

// IncrementCount increments the message count and updates last_used.
func (db *DB) IncrementCount(chatID int64) error {
	_, err := db.Exec(`
		UPDATE user_sessions
		SET message_count = message_count + 1, last_used = CURRENT_TIMESTAMP
		WHERE chat_id = ?`, chatID)
	return err
}

// ListAll returns all sessions ordered by last_used descending.
func (db *DB) ListAll() ([]Session, error) {
	rows, err := db.Query(`
		SELECT chat_id, session_id, title, agent, model_provider, model_id, message_count, created_at, last_used
		FROM user_sessions ORDER BY last_used DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		var agent sql.NullString
		var modelProvider sql.NullString
		var modelID sql.NullString
		if err := rows.Scan(&s.ChatID, &s.SessionID, &s.Title, &agent, &modelProvider, &modelID, &s.MessageCount, &s.CreatedAt, &s.LastUsed); err != nil {
			log.Printf("Error scanning session: %v", err)
			continue
		}
		s.Agent = agent.String
		s.ModelProvider = modelProvider.String
		s.ModelID = modelID.String
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// DeleteAll removes all sessions (for purge).
func (db *DB) DeleteAll() error {
	_, err := db.Exec(`DELETE FROM user_sessions`)
	return err
}
