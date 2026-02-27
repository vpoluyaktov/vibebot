package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Memory manages the two-layer memory system
type Memory struct {
	workspaceDir string
	memoryFile   string
	historyFile  string
	projectsDir  string
}

// New creates a new Memory instance
func New(workspaceDir string) (*Memory, error) {
	memoryDir := filepath.Join(workspaceDir, "memory")
	projectsDir := filepath.Join(workspaceDir, "projects")
	
	// Create memory directory if it doesn't exist
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return nil, err
	}
	
	// Create projects directory if it doesn't exist
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		return nil, err
	}
	
	m := &Memory{
		workspaceDir: workspaceDir,
		memoryFile:   filepath.Join(memoryDir, "GlobalMemory.md"),
		historyFile:  filepath.Join(memoryDir, "HISTORY.md"),
		projectsDir:  projectsDir,
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

// GetMemoryPath returns the path to GlobalMemory.md
func (m *Memory) GetMemoryPath() string {
	return m.memoryFile
}

// GetHistoryPath returns the path to HISTORY.md
func (m *Memory) GetHistoryPath() string {
	return m.historyFile
}

// GetWorkspaceDir returns the workspace directory path
func (m *Memory) GetWorkspaceDir() string {
	return m.workspaceDir
}

// ValidateProjectName validates a project name for safety
func (m *Memory) ValidateProjectName(name string) error {
	if len(name) == 0 || len(name) > 64 {
		return fmt.Errorf("project name must be 1-64 characters")
	}
	
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	if !matched {
		return fmt.Errorf("project name can only contain letters, numbers, hyphens, and underscores")
	}
	
	return nil
}

// GetProjectPath returns the file path for a project
func (m *Memory) GetProjectPath(projectName string) string {
	return filepath.Join(m.projectsDir, projectName+".md")
}

// LoadProjectMemory reads a project memory file
func (m *Memory) LoadProjectMemory(projectName string) (string, error) {
	if err := m.ValidateProjectName(projectName); err != nil {
		return "", err
	}
	
	projectPath := m.GetProjectPath(projectName)
	data, err := os.ReadFile(projectPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("project '%s' does not exist", projectName)
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveProjectMemory writes to a project memory file
func (m *Memory) SaveProjectMemory(projectName, content string) error {
	if err := m.ValidateProjectName(projectName); err != nil {
		return err
	}
	
	projectPath := m.GetProjectPath(projectName)
	return os.WriteFile(projectPath, []byte(content), 0644)
}

// CreateProject creates a new project with a template
func (m *Memory) CreateProject(projectName string) error {
	if err := m.ValidateProjectName(projectName); err != nil {
		return err
	}
	
	projectPath := m.GetProjectPath(projectName)
	
	// Check if project already exists
	if _, err := os.Stat(projectPath); err == nil {
		return fmt.Errorf("project '%s' already exists", projectName)
	}
	
	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	template := fmt.Sprintf(`# Project: %s

## Description
[Brief project description]

## Status
Active

## Key Facts
- Created: %s
- 

## Current Focus
[What you're currently working on]

## Notes
[Detailed project-specific context, decisions, architecture notes, etc.]

## Links
- [Related documentation]
- [Issue trackers]
- [Deployment URLs]
`, projectName, timestamp)
	
	return os.WriteFile(projectPath, []byte(template), 0644)
}

// ListProjects returns a list of all project names
func (m *Memory) ListProjects() ([]string, error) {
	entries, err := os.ReadDir(m.projectsDir)
	if err != nil {
		return nil, err
	}
	
	var projects []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			// Remove .md extension
			projectName := strings.TrimSuffix(entry.Name(), ".md")
			projects = append(projects, projectName)
		}
	}
	
	return projects, nil
}

// DeleteProject deletes a project memory file
func (m *Memory) DeleteProject(projectName string) error {
	if err := m.ValidateProjectName(projectName); err != nil {
		return err
	}
	
	projectPath := m.GetProjectPath(projectName)
	
	// Check if project exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return fmt.Errorf("project '%s' does not exist", projectName)
	}
	
	return os.Remove(projectPath)
}

// ProjectExists checks if a project exists
func (m *Memory) ProjectExists(projectName string) bool {
	if err := m.ValidateProjectName(projectName); err != nil {
		return false
	}
	
	projectPath := m.GetProjectPath(projectName)
	_, err := os.Stat(projectPath)
	return err == nil
}