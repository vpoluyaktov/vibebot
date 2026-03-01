package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/tasks"
)

var messageSender MessageSender
var taskManager *tasks.Manager

// RegisterTimerTool registers the timer tool with a message sender and task manager
func RegisterTimerTool(registry *Registry, sender MessageSender, tm *tasks.Manager) {
	messageSender = sender
	taskManager = tm
	
	registry.Register("set_timer", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "set_timer",
				Description: "Set a timer that triggers automatic task resumption after specified seconds. Use this for long-running operations (builds, downloads, etc.) where you want to check results automatically when complete.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"duration": map[string]interface{}{
							"type":        "number",
							"description": "Number of seconds to wait before resumption",
						},
						"task_context": map[string]interface{}{
							"type":        "string",
							"description": "Description of what to do when timer expires (e.g., 'Check if build completed and report results')",
						},
						"message": map[string]interface{}{
							"type":        "string",
							"description": "Optional notification message to send when timer expires (default: based on task_context)",
						},
					},
					"required": []string{"duration", "task_context"},
				},
			},
		},
		Handler: setTimerHandler,
	})
}

// setTimerHandler implements a timer that triggers automatic task resumption
func setTimerHandler(ctx context.Context, args map[string]interface{}) (string, error) {
	duration, ok := args["duration"].(float64)
	if !ok {
		return "", fmt.Errorf("duration must be a number")
	}

	taskContext, ok := args["task_context"].(string)
	if !ok || taskContext == "" {
		return "", fmt.Errorf("task_context is required")
	}

	message, _ := args["message"].(string)
	if message == "" {
		message = fmt.Sprintf("⏰ Timer expired! Resuming task: %s", taskContext)
	}

	// Get chat ID from context
	var chatID int64
	if ctxChatID := ctx.Value("chat_id"); ctxChatID != nil {
		chatID = ctxChatID.(int64)
	}

	if chatID == 0 {
		return "", fmt.Errorf("chat_id not found in context")
	}

	if messageSender == nil {
		return "", fmt.Errorf("message sender not initialized")
	}

	if taskManager == nil {
		return "", fmt.Errorf("task manager not initialized")
	}

	// Create pending task
	taskID, err := taskManager.CreateTask(
		chatID,
		message,
		taskContext,
		time.Duration(duration)*time.Second,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create task: %w", err)
	}

	return fmt.Sprintf("✅ Timer set for %.0f seconds. Task will resume automatically: \"%s\" (ID: %s)", 
		duration, taskContext, taskID), nil
}
