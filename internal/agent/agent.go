package agent

import (
	"context"
	"fmt"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
	"github.com/vpoluyaktov/vibebot/internal/memory"
	"github.com/vpoluyaktov/vibebot/internal/session"
	"github.com/vpoluyaktov/vibebot/internal/tools"
)

const maxToolIterations = 40 // Prevent infinite loops (matches nanobot default)
const maxHistoryMessages = 50 // Maximum messages to include in context

// Agent represents the core AI agent
type Agent struct {
	llm      llm.Provider
	memory   *memory.Memory
	tools    *tools.Registry
	sessions *session.Manager
}

// New creates a new Agent instance
func New(provider llm.Provider, mem *memory.Memory, toolRegistry *tools.Registry, sessionMgr *session.Manager) *Agent {
	return &Agent{
		llm:      provider,
		memory:   mem,
		tools:    toolRegistry,
		sessions: sessionMgr,
	}
}

// ProcessMessage handles an incoming message and generates a response
func (a *Agent) ProcessMessage(ctx context.Context, chatID int64, message string) (string, error) {
	logger.Debug("Processing message from chat %d: %s", chatID, message)

	// Add chat_id to context for tools
	ctx = context.WithValue(ctx, "chat_id", chatID)

	// Handle commands
	if message == "/new" || message == "/start" {
		sess := a.sessions.GetOrCreate(chatID)
		sess.Clear()
		if err := a.sessions.Save(sess); err != nil {
			logger.Warn("Failed to save cleared session: %v", err)
		}
		return "🔄 New conversation started. Previous context cleared.", nil
	}

	if message == "/help" {
		return "🤖 **vibebot** - AI Assistant\n\n" +
			"**Commands:**\n" +
			"/new - Start a new conversation (clears context)\n" +
			"/help - Show this help message\n\n" +
			"Just send me a message and I'll help you!", nil
	}

	// Get or create session for this chat
	sess := a.sessions.GetOrCreate(chatID)

	// Add user message to session
	sess.AddMessage(llm.Message{
		Role:    "user",
		Content: message,
	})

	// Load memory context
	memoryContent, err := a.memory.LoadMemory()
	if err != nil {
		logger.Warn("Failed to load memory: %v", err)
		memoryContent = ""
	}

	// Build system message
	systemMessage := systemPrompt
	if memoryContent != "" {
		systemMessage += "\n\n## Current Memory (MEMORY.md)\n\n" + memoryContent
	}

	// Build messages array: system + conversation history
	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemMessage,
		},
	}

	// Add conversation history from session
	history := sess.GetHistory(maxHistoryMessages)
	messages = append(messages, history...)

	// Agent loop: handle tool calls iteratively
	var finalResponse string
	for i := 0; i < maxToolIterations; i++ {
		// Call LLM with available tools
		response, err := a.llm.Chat(ctx, messages, a.tools.GetDefinitions())
		if err != nil {
			return "", fmt.Errorf("LLM error: %w", err)
		}

		// If no tool calls, we're done
		if len(response.ToolCalls) == 0 {
			finalResponse = response.Content
			break
		}

		// Add assistant message with tool calls
		assistantMsg := llm.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		}
		messages = append(messages, assistantMsg)
		sess.AddMessage(assistantMsg)

		// Execute each tool call
		for _, toolCall := range response.ToolCalls {
			logger.Debug("Executing tool: %s", toolCall.Function.Name)

			result, err := a.tools.Execute(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
				logger.Error("Tool execution error: %v", err)
			}

			// Add tool result as a message
			toolMsg := llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
			}
			messages = append(messages, toolMsg)
			sess.AddMessage(toolMsg)
		}

		// Continue loop to let LLM process tool results
	}

	// Check if we hit max iterations
	if finalResponse == "" {
		logger.Warn("Max tool iterations (%d) reached for chat %d", maxToolIterations, chatID)
		finalResponse = fmt.Sprintf(
			"I reached the maximum number of tool call iterations (%d) without completing the task. "+
				"You can try breaking the task into smaller steps or rephrasing your request.",
			maxToolIterations,
		)
	}

	// Add final assistant response to session
	sess.AddMessage(llm.Message{
		Role:    "assistant",
		Content: finalResponse,
	})

	// Save session to disk
	if err := a.sessions.Save(sess); err != nil {
		logger.Warn("Failed to save session: %v", err)
	}

	// Log conversation to history
	if err := a.memory.LogConversation(chatID, message, finalResponse); err != nil {
		logger.Warn("Failed to log conversation: %v", err)
	}

	return finalResponse, nil
}
