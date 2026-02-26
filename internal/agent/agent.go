package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/memory"
)

// Agent represents the core AI agent
type Agent struct {
	llm    llm.Provider
	memory *memory.Memory
}

// New creates a new Agent instance
func New(provider llm.Provider, mem *memory.Memory) *Agent {
	return &Agent{
		llm:    provider,
		memory: mem,
	}
}

// ProcessMessage handles an incoming message and generates a response
func (a *Agent) ProcessMessage(ctx context.Context, chatID int64, message string) (string, error) {
	log.Printf("Processing message from chat %d: %s", chatID, message)

	// Load memory context
	memoryContent, err := a.memory.LoadMemory()
	if err != nil {
		log.Printf("Warning: failed to load memory: %v", err)
		memoryContent = ""
	}

	// Build messages for LLM
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

	// Call LLM (no tools for now, we'll add them next)
	response, err := a.llm.Chat(ctx, messages, nil)
	if err != nil {
		return "", fmt.Errorf("LLM error: %w", err)
	}

	// TODO: Handle tool calls when we implement them
	if len(response.ToolCalls) > 0 {
		log.Printf("Tool calls received but not yet implemented: %d calls", len(response.ToolCalls))
	}

	return response.Content, nil
}
