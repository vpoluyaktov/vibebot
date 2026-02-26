package modelmanager

import (
	"fmt"
	"sync"
)

// Manager handles model selection and validation
type Manager struct {
	mu            sync.RWMutex
	currentModel  string
	allowedModels []string
}

// New creates a new model manager
func New(currentModel string, allowedModels []string) *Manager {
	return &Manager{
		currentModel:  currentModel,
		allowedModels: allowedModels,
	}
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
