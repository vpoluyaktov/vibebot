package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Memory manages the two-layer memory system
type Memory struct {
	workspaceDir string
	memoryFile   string
	historyFile  string
}

// New creates a new Memory instance
func New(workspaceDir string) (*Memory, error) {
	memoryDir := filepath.Join(workspaceDir, "memory")
	
	// Create memory directory if it doesn't exist
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return nil, err
	}
	
	m := &Memory{
		workspaceDir: workspaceDir,
		memoryFile:   filepath.Join(memoryDir, "MEMORY.md"),
		historyFile:  filepath.Join(memoryDir, "HISTORY.md"),
	}
	
	// Initialize MEMORY.md if it doesn't exist
	if err := m.initializeMemoryFile(); err != nil {
		return nil, err
	}
	
	return m, nil
}

// initializeMemoryFile creates MEMORY.md with a template if it doesn't exist
func (m *Memory) initializeMemoryFile() error {
	if _, err := os.Stat(m.memoryFile); os.IsNotExist(err) {
		template := `# Long-term Memory

This file stores important information that should persist across sessions.

## User Information

- **Name**: (not set)
- **Timezone**: (not set)
- **Preferences**: (not set)

## Important Facts

(Add important facts here)

## Active Projects

(Add project information here)

---

*This file is automatically loaded into context and can be edited by the bot.*
`
		return os.WriteFile(m.memoryFile, []byte(template), 0644)
	}
	return nil
}

// LoadMemory reads the long-term memory file
func (m *Memory) LoadMemory() (string, error) {
	data, err := os.ReadFile(m.memoryFile)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveMemory writes to the long-term memory file
func (m *Memory) SaveMemory(content string) error {
	return os.WriteFile(m.memoryFile, []byte(content), 0644)
}

// AppendHistory adds an entry to the history log
func (m *Memory) AppendHistory(entry string) error {
	f, err := os.OpenFile(m.historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	
	_, err = f.WriteString(entry + "\n")
	return err
}

// LogConversation logs a user message and bot response to history
func (m *Memory) LogConversation(chatID int64, userMessage, botResponse string) error {
	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	entry := fmt.Sprintf(`
---
[%s] Chat: %d

User: %s

Bot: %s
`, timestamp, chatID, userMessage, botResponse)
	
	return m.AppendHistory(entry)
}

// GetMemoryPath returns the path to MEMORY.md
func (m *Memory) GetMemoryPath() string {
	return m.memoryFile
}

// GetHistoryPath returns the path to HISTORY.md
func (m *Memory) GetHistoryPath() string {
	return m.historyFile
}