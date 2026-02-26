package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/memory"
	"github.com/vpoluyaktov/vibebot/internal/tools"
)

const maxToolIterations = 10 // Prevent infinite loops

// Agent represents the core AI agent
type Agent struct {
	llm      llm.Provider
	memory   *memory.Memory
	tools    *tools.Registry
}

// New creates a new Agent instance
func New(provider llm.Provider, mem *memory.Memory, toolRegistry *tools.Registry) *Agent {
	return &Agent{
		llm:    provider,
		memory: mem,
		tools:  toolRegistry,
	}
}

// ProcessMessage handles an incoming message and generates a response
func (a *Agent) ProcessMessage(ctx context.Context, chatID int64, message string) (string, error) {
	log.Printf("Processing message from chat %d: %s", chatID, message)

	// Add chat_id to context for tools
	ctx = context.WithValue(ctx, "chat_id", chatID)

	// Load memory context
	memoryContent, err := a.memory.LoadMemory()
	if err != nil {
		log.Printf("Warning: failed to load memory: %v", err)
		memoryContent = ""
	}

	// Build initial messages
	messages := []llm.Message{
		{
			Role: "system",
			Content: `You are vibebot, a helpful AI assistant written in Go.

You have access to a persistent memory system. Important facts are stored in MEMORY.md.

Current memory:
` + memoryContent,
		},
		{
			Role:    "user",
			Content: message,
		},
	}

	// Agent loop: handle tool calls iteratively
	for i := 0; i < maxToolIterations; i++ {
		// Call LLM with available tools
		response, err := a.llm.Chat(ctx, messages, a.tools.GetDefinitions())
		if err != nil {
			return "", fmt.Errorf("LLM error: %w", err)
		}

		// If no tool calls, we're done
		if len(response.ToolCalls) == 0 {
			return response.Content, nil
		}

		// Add assistant message with tool calls
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})

		// Execute each tool call
		for _, toolCall := range response.ToolCalls {
			log.Printf("Executing tool: %s", toolCall.Function.Name)

			result, err := a.tools.Execute(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
				log.Printf("Tool execution error: %v", err)
			}

			// Add tool result as a message
			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}

		// Continue loop to let LLM process tool results
	}

	return "", fmt.Errorf("max tool iterations reached")
}
