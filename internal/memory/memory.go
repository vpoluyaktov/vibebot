package memory

import (
	"os"
	"path/filepath"
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
	
	return &Memory{
		workspaceDir: workspaceDir,
		memoryFile:   filepath.Join(memoryDir, "MEMORY.md"),
		historyFile:  filepath.Join(memoryDir, "HISTORY.md"),
	}, nil
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
