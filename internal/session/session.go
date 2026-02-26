package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
)

// Session represents a conversation session with message history
type Session struct {
	Key              string         `json:"key"`
	Messages         []llm.Message  `json:"messages"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	LastConsolidated int            `json:"last_consolidated"`
	mu               sync.RWMutex
}

// Manager manages conversation sessions
type Manager struct {
	workspaceDir string
	sessionsDir  string
	cache        map[string]*Session
	mu           sync.RWMutex
}

// NewManager creates a new session manager
func NewManager(workspaceDir string) (*Manager, error) {
	sessionsDir := filepath.Join(workspaceDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &Manager{
		workspaceDir: workspaceDir,
		sessionsDir:  sessionsDir,
		cache:        make(map[string]*Session),
	}, nil
}

// GetOrCreate retrieves an existing session or creates a new one
func (m *Manager) GetOrCreate(chatID int64) *Session {
	key := fmt.Sprintf("telegram:%d", chatID)

	m.mu.RLock()
	if session, exists := m.cache[key]; exists {
		m.mu.RUnlock()
		return session
	}
	m.mu.RUnlock()

	// Try to load from disk
	session := m.load(key)
	if session == nil {
		// Create new session
		session = &Session{
			Key:              key,
			Messages:         []llm.Message{},
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
			LastConsolidated: 0,
		}
	}

	m.mu.Lock()
	m.cache[key] = session
	m.mu.Unlock()

	return session
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(msg llm.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// GetHistory returns recent messages for LLM context
func (s *Session) GetHistory(maxMessages int) []llm.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get unconsolidated messages
	unconsolidated := s.Messages[s.LastConsolidated:]
	
	// Limit to maxMessages
	start := 0
	if len(unconsolidated) > maxMessages {
		start = len(unconsolidated) - maxMessages
	}
	
	sliced := unconsolidated[start:]

	// Drop leading non-user messages to avoid orphaned tool results
	for i, m := range sliced {
		if m.Role == "user" {
			return sliced[i:]
		}
	}

	return sliced
}

// Clear clears all messages in the session
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = []llm.Message{}
	s.LastConsolidated = 0
	s.UpdatedAt = time.Now()
}

// Save persists the session to disk
func (m *Manager) Save(session *Session) error {
	session.mu.RLock()
	defer session.mu.RUnlock()

	path := m.getSessionPath(session.Key)

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	logger.Debug("Session saved: %s", session.Key)
	return nil
}

// load loads a session from disk
func (m *Manager) load(key string) *Session {
	path := m.getSessionPath(key)

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read session file %s: %v", path, err)
		}
		return nil
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		logger.Warn("Failed to unmarshal session %s: %v", key, err)
		return nil
	}

	logger.Debug("Session loaded: %s (%d messages)", key, len(session.Messages))
	return &session
}

// getSessionPath returns the file path for a session
func (m *Manager) getSessionPath(key string) string {
	// Replace colons with underscores for filename safety
	safeKey := filepath.Base(key)
	safeKey = filepath.Clean(safeKey)
	return filepath.Join(m.sessionsDir, safeKey+".json")
}
