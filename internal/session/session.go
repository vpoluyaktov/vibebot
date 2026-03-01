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
	Key              string        `json:"key"`
	Messages         []llm.Message `json:"messages"`
	CurrentProject   string        `json:"current_project,omitempty"`
	ShowVerbose      bool          `json:"show_verbose"`
	ShowStats        bool          `json:"show_stats"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	LastConsolidated int           `json:"last_consolidated"`
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

// GetHistory returns recent messages for LLM context with smart compression
// Recent messages (last 5) include full tool details
// Older messages compress tool results to save tokens
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

	messages := unconsolidated[start:]

	// Apply compression to older messages
	// Keep last 5 messages with full detail, compress older ones
	const fullDetailWindow = 5
	result := make([]llm.Message, 0, len(messages))

	for i, msg := range messages {
		isRecent := i >= len(messages)-fullDetailWindow

		if isRecent {
			// Recent messages: include everything
			result = append(result, msg)
		} else if msg.Role == "tool" {
			// Older tool results: compress to summary
			compressed := msg
			if len(msg.Content) > 200 {
				compressed.Content = msg.Content[:200] + "... [truncated]"
			}
			result = append(result, compressed)
		} else if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			// Older assistant with tool calls: keep structure but note compression
			result = append(result, msg)
		} else {
			// User messages and final assistant responses: always include
			result = append(result, msg)
		}
	}

	return result
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

// SetProject sets the current project for the session
func (s *Session) SetProject(projectName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.CurrentProject = projectName
	s.UpdatedAt = time.Now()
}

// GetProject returns the current project name (empty string if none)
func (s *Session) GetProject() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.CurrentProject
}

// ClearProject clears the current project
func (s *Session) ClearProject() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.CurrentProject = ""
	s.UpdatedAt = time.Now()
}

// NeedsConsolidation checks if the session exceeds the consolidation threshold
func (s *Session) NeedsConsolidation(threshold int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.Messages) > threshold
}

// GetMessagesForConsolidation returns messages that need to be consolidated
func (s *Session) GetMessagesForConsolidation(batchSize int) []llm.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.LastConsolidated >= len(s.Messages) {
		return nil
	}

	end := s.LastConsolidated + batchSize
	if end > len(s.Messages) {
		end = len(s.Messages)
	}

	// Return unconsolidated messages
	return s.Messages[s.LastConsolidated:end]
}

// MarkConsolidated marks messages as consolidated
func (s *Session) MarkConsolidated(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.LastConsolidated += count
	if s.LastConsolidated > len(s.Messages) {
		s.LastConsolidated = len(s.Messages)
	}
	s.UpdatedAt = time.Now()
}

// PruneConsolidated removes consolidated messages to save space
// This is optional and can be enabled later if needed
func (s *Session) PruneConsolidated() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.LastConsolidated > 0 && s.LastConsolidated < len(s.Messages) {
		// Keep only unconsolidated messages
		s.Messages = s.Messages[s.LastConsolidated:]
		s.LastConsolidated = 0
		s.UpdatedAt = time.Now()
	}
}

// GetSession retrieves a session from cache or disk
func (m *Manager) GetSession(key string) (*Session, error) {
	m.mu.RLock()
	if session, exists := m.cache[key]; exists {
		m.mu.RUnlock()
		return session, nil
	}
	m.mu.RUnlock()

	// Try to load from disk
	session := m.load(key)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", key)
	}

	m.mu.Lock()
	m.cache[key] = session
	m.mu.Unlock()

	return session, nil
}

// SaveSession is an alias for Save for consistency
func (m *Manager) SaveSession(session *Session) error {
	return m.Save(session)
}

// SetShowVerbose sets the verbose display preference
func (s *Session) SetShowVerbose(show bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ShowVerbose = show
	s.UpdatedAt = time.Now()
}

// GetShowVerbose returns the verbose display preference
func (s *Session) GetShowVerbose() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.ShowVerbose
}

// SetShowStats sets the stats display preference
func (s *Session) SetShowStats(show bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ShowStats = show
	s.UpdatedAt = time.Now()
}

// GetShowStats returns the stats display preference
func (s *Session) GetShowStats() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.ShowStats
}
