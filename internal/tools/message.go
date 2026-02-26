package tools

import (
	"context"
	"fmt"

	"github.com/vpoluyaktov/vibebot/internal/llm"
)

// MessageSender is an interface for sending messages
type MessageSender interface {
	SendMessage(chatID int64, text string) error
}

// RegisterMessageTool adds the message tool to the registry
func RegisterMessageTool(registry *Registry, sender MessageSender) {
	registry.Register("message", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "message",
				Description: "Send a message to the user. Use this when you want to communicate something.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"content": map[string]interface{}{
							"type":        "string",
							"description": "The message content to send",
						},
						"chat_id": map[string]interface{}{
							"type":        "string",
							"description": "Optional: target chat ID (defaults to current chat)",
						},
					},
					"required": []string{"content"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			content, ok := args["content"].(string)
			if !ok {
				return "", fmt.Errorf("content must be a string")
			}

			// Get chat ID from context or args
			var chatID int64
			if chatIDStr, ok := args["chat_id"].(string); ok && chatIDStr != "" {
				// Parse chat ID from string
				fmt.Sscanf(chatIDStr, "%d", &chatID)
			} else {
				// Get from context
				if ctxChatID := ctx.Value("chat_id"); ctxChatID != nil {
					chatID = ctxChatID.(int64)
				}
			}

			if chatID == 0 {
				return "", fmt.Errorf("chat_id not provided and not in context")
			}

			if err := sender.SendMessage(chatID, content); err != nil {
				return "", fmt.Errorf("failed to send message: %w", err)
			}

			return "Message sent successfully", nil
		},
	})
}
