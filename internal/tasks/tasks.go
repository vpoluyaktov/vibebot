package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PendingTask represents a task waiting for timer completion
type PendingTask struct {
	ID          string    `json:"id"`
	ChatID      int64     `json:"chat_id"`
	Description string    `json:"description"`
	Context     string    `json:"context"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// TaskResumer is a callback function to resume task execution
type TaskResumer func(ctx context.Context, chatID int64, taskContext string) error

// Manager handles pending tasks
type Manager struct {
	mu            sync.RWMutex
	tasks         map[string]*PendingTask
	storageDir    string
	resumeHandler TaskResumer
}

// NewManager creates a new task manager
func NewManager(storageDir string) *Manager {
	m := &Manager{
		tasks:      make(map[string]*PendingTask),
		storageDir: storageDir,
	}
	
	// Ensure storage directory exists
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		fmt.Printf("Failed to create tasks directory: %v\n", err)
	}
	
	// Load existing tasks
	m.loadTasks()
	
	return m
}

// SetResumeHandler sets the callback function for task resumption
func (m *Manager) SetResumeHandler(handler TaskResumer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resumeHandler = handler
}

// CreateTask creates a new pending task
func (m *Manager) CreateTask(chatID int64, description string, context string, duration time.Duration) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	taskID := fmt.Sprintf("task_%d_%d", chatID, time.Now().UnixNano())
	task := &PendingTask{
		ID:          taskID,
		ChatID:      chatID,
		Description: description,
		Context:     context,
		ExpiresAt:   time.Now().Add(duration),
		CreatedAt:   time.Now(),
	}
	
	m.tasks[taskID] = task
	
	// Save to disk
	if err := m.saveTask(task); err != nil {
		return "", fmt.Errorf("failed to save task: %w", err)
	}
	
	// Schedule timer
	go m.scheduleTask(task)
	
	return taskID, nil
}

// scheduleTask schedules a task to execute when timer expires
func (m *Manager) scheduleTask(task *PendingTask) {
	duration := time.Until(task.ExpiresAt)
	if duration < 0 {
		duration = 0
	}
	
	time.Sleep(duration)
	
	// Task expired, resume it
	m.mu.RLock()
	handler := m.resumeHandler
	m.mu.RUnlock()
	
	if handler != nil {
		ctx := context.Background()
		ctx = context.WithValue(ctx, "chat_id", task.ChatID)
		
		if err := handler(ctx, task.ChatID, task.Context); err != nil {
			fmt.Printf("Failed to resume task %s: %v\n", task.ID, err)
		}
	}
	
	// Clean up task
	m.deleteTask(task.ID)
}

// deleteTask removes a task from storage
func (m *Manager) deleteTask(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.tasks, taskID)
	
	taskFile := filepath.Join(m.storageDir, taskID+".json")
	os.Remove(taskFile)
}

// saveTask saves a task to disk
func (m *Manager) saveTask(task *PendingTask) error {
	taskFile := filepath.Join(m.storageDir, task.ID+".json")
	
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(taskFile, data, 0644)
}

// loadTasks loads all tasks from disk
func (m *Manager) loadTasks() {
	files, err := os.ReadDir(m.storageDir)
	if err != nil {
		return
	}
	
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}
		
		taskFile := filepath.Join(m.storageDir, file.Name())
		data, err := os.ReadFile(taskFile)
		if err != nil {
			continue
		}
		
		var task PendingTask
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}
		
		// Skip expired tasks
		if time.Now().After(task.ExpiresAt) {
			os.Remove(taskFile)
			continue
		}
		
		m.tasks[task.ID] = &task
		
		// Reschedule task
		go m.scheduleTask(&task)
	}
}

// GetTask retrieves a task by ID
func (m *Manager) GetTask(taskID string) (*PendingTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	task, exists := m.tasks[taskID]
	return task, exists
}

// ListTasks returns all active tasks for a chat
func (m *Manager) ListTasks(chatID int64) []*PendingTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var result []*PendingTask
	for _, task := range m.tasks {
		if task.ChatID == chatID {
			result = append(result, task)
		}
	}
	
	return result
}
