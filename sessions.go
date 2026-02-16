package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Session represents a user's session in the database
type Session struct {
	ChatID       int64
	SessionID    string
	Title        string
	MessageCount int
	CreatedAt    time.Time
	LastUsed     time.Time
}

// DB handles SQLite database operations for session management
type DB struct {
	*sql.DB
}

// NewDB creates a new database connection and initializes the schema
func NewDB(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	database := &DB{db}
	if err := database.Init(); err != nil {
		db.Close()
		return nil, err
	}

	return database, nil
}

// Init creates the user_sessions table if it doesn't exist
func (db *DB) Init() error {
	query := `
		CREATE TABLE IF NOT EXISTS user_sessions (
			chat_id INTEGER PRIMARY KEY,
			session_id TEXT NOT NULL,
			title TEXT,
			message_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_used DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`

	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Error creating user_sessions table: %v", err)
		return err
	}

	log.Println("Database initialized successfully")
	return nil
}

// GetSession retrieves a session by chat ID
func (db *DB) GetSession(chatID int64) (Session, error) {
	query := `
		SELECT chat_id, session_id, title, message_count, created_at, last_used
		FROM user_sessions
		WHERE chat_id = ?
	`

	var session Session
	err := db.QueryRow(query, chatID).Scan(
		&session.ChatID,
		&session.SessionID,
		&session.Title,
		&session.MessageCount,
		&session.CreatedAt,
		&session.LastUsed,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return Session{}, err
		}
		log.Printf("Error getting session for chat_id %d: %v", chatID, err)
		return Session{}, err
	}

	return session, nil
}

// SetSession inserts or replaces a user session (upsert)
func (db *DB) SetSession(session Session) error {
	query := `
		INSERT OR REPLACE INTO user_sessions (chat_id, session_id, title, message_count, created_at, last_used)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		session.ChatID,
		session.SessionID,
		session.Title,
		session.MessageCount,
		session.CreatedAt,
		session.LastUsed,
	)

	if err != nil {
		log.Printf("Error setting session for chat_id %d: %v", session.ChatID, err)
		return err
	}

	return nil
}

// DeleteSession removes a session by chat ID
func (db *DB) DeleteSession(chatID int64) error {
	query := `DELETE FROM user_sessions WHERE chat_id = ?`

	result, err := db.Exec(query, chatID)
	if err != nil {
		log.Printf("Error deleting session for chat_id %d: %v", chatID, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		log.Printf("No session found to delete for chat_id %d", chatID)
	}

	return nil
}

// IncrementCount increments the message_count for a session
func (db *DB) IncrementCount(chatID int64) error {
	query := `
		UPDATE user_sessions
		SET message_count = message_count + 1, last_used = CURRENT_TIMESTAMP
		WHERE chat_id = ?
	`

	result, err := db.Exec(query, chatID)
	if err != nil {
		log.Printf("Error incrementing count for chat_id %d: %v", chatID, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		log.Printf("No session found to increment count for chat_id %d", chatID)
	}

	return nil
}

// ListAll retrieves all user sessions
func (db *DB) ListAll() ([]Session, error) {
	query := `
		SELECT chat_id, session_id, title, message_count, created_at, last_used
		FROM user_sessions
		ORDER BY last_used DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Error listing all sessions: %v", err)
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var session Session
		err := rows.Scan(
			&session.ChatID,
			&session.SessionID,
			&session.Title,
			&session.MessageCount,
			&session.CreatedAt,
			&session.LastUsed,
		)
		if err != nil {
			log.Printf("Error scanning session: %v", err)
			continue
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating sessions: %v", err)
		return nil, err
	}

	return sessions, nil
}
