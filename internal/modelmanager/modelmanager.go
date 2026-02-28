package modelmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Manager handles model selection and validation
type Manager struct {
	mu             sync.RWMutex
	currentModel   string
	allowedModels  []string
	contextLengths map[string]int // model ID -> context length
	stateFile      string
}

// New creates a new model manager
func New(currentModel string, allowedModels []string, workspaceDir string) *Manager {
	stateFile := filepath.Join(workspaceDir, "memory", "current_model.txt")
	
	m := &Manager{
		currentModel:   currentModel,
		allowedModels:  allowedModels,
		contextLengths: make(map[string]int),
		stateFile:      stateFile,
	}
	
	// Try to load saved model preference
	if savedModel, err := m.loadSavedModel(); err == nil && savedModel != "" {
		// Only use saved model if it's in the allowed list
		if m.isAllowed(savedModel) {
			m.currentModel = savedModel
		}
	}
	
	return m
}

// GetCurrent returns the current model name
func (m *Manager) GetCurrent() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentModel
}

// SetCurrent sets the current model if it's in the allowed list
func (m *Manager) SetCurrent(model string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.isAllowed(model) {
		return fmt.Errorf("model '%s' is not in the allowed list", model)
	}
	
	m.currentModel = model
	
	// Save the model selection to disk (ignore errors, non-critical)
	_ = m.saveModel(model)
	
	return nil
}

// GetAllowed returns the list of allowed models
func (m *Manager) GetAllowed() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a copy to prevent external modification
	models := make([]string, len(m.allowedModels))
	copy(models, m.allowedModels)
	return models
}

// isAllowed checks if a model is in the allowed list (must be called with lock held)
func (m *Manager) isAllowed(model string) bool {
	for _, allowed := range m.allowedModels {
		if allowed == model {
			return true
		}
	}
	return false
}

// saveModel persists the current model selection to disk
func (m *Manager) saveModel(model string) error {
	// Ensure the directory exists
	dir := filepath.Dir(m.stateFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	
	// Write the model name to the state file
	if err := os.WriteFile(m.stateFile, []byte(model), 0644); err != nil {
		return fmt.Errorf("failed to save model state: %w", err)
	}
	
	return nil
}

// loadSavedModel reads the saved model selection from disk
func (m *Manager) loadSavedModel() (string, error) {
	data, err := os.ReadFile(m.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No saved state, not an error
		}
		return "", fmt.Errorf("failed to read model state: %w", err)
	}
	
	// Trim whitespace and return the model name
	return strings.TrimSpace(string(data)), nil
}

// SetContextLengths sets the context lengths for models (called during initialization)
func (m *Manager) SetContextLengths(contextLengths map[string]int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contextLengths = contextLengths
}

// GetContextLength returns the context length for the current model, or 0 if unknown
func (m *Manager) GetContextLength() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if length, ok := m.contextLengths[m.currentModel]; ok {
		return length
	}
	return 0
}
