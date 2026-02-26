package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
	"github.com/vpoluyaktov/vibebot/internal/memory"
	"github.com/vpoluyaktov/vibebot/internal/modelmanager"
	"github.com/vpoluyaktov/vibebot/internal/session"
	"github.com/vpoluyaktov/vibebot/internal/tools"
)

const maxToolIterations = 40 // Prevent infinite loops (matches nanobot default)
const maxHistoryMessages = 50 // Maximum messages to include in context

// Agent represents the core AI agent
type Agent struct {
	llm          llm.Provider
	memory       *memory.Memory
	tools        *tools.Registry
	sessions     *session.Manager
	modelManager *modelmanager.Manager
}

// New creates a new Agent instance
func New(provider llm.Provider, mem *memory.Memory, toolRegistry *tools.Registry, sessionMgr *session.Manager, modelMgr *modelmanager.Manager) *Agent {
	return &Agent{
		llm:          provider,
		memory:       mem,
		tools:        toolRegistry,
		sessions:     sessionMgr,
		modelManager: modelMgr,
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
			"/model - Show current LLM model\n" +
			"/model list - List all available models (numbered)\n" +
			"/model <number> - Switch by number (e.g., `/model 2`)\n" +
			"/model <partial-name> - Switch by name (e.g., `/model llama`)\n" +
			"/help - Show this help message\n\n" +
			"Just send me a message and I'll help you!", nil
	}

	// Handle /model commands
	if strings.HasPrefix(message, "/model") {
		return a.handleModelCommand(message)
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

// handleModelCommand processes /model commands
func (a *Agent) handleModelCommand(message string) (string, error) {
	parts := strings.Fields(message)
	
	// /model or /model list - show all models
	if len(parts) == 1 || (len(parts) == 2 && parts[1] == "list") {
		current := a.modelManager.GetCurrent()
		allowed := a.modelManager.GetAllowed()
		
		var response strings.Builder
		response.WriteString("🤖 **Available Models**\n\n")
		
		for i, model := range allowed {
			if model == current {
				response.WriteString(fmt.Sprintf("%d. ✅ `%s` _(current)_\n", i+1, model))
			} else {
				response.WriteString(fmt.Sprintf("%d.    `%s`\n", i+1, model))
			}
		}
		
		response.WriteString("\n💡 Use `/model <number>` or `/model <partial-name>` to switch")
		return response.String(), nil
	}
	
	// /model <number or partial name> - switch model
	if len(parts) >= 2 {
		selector := strings.Join(parts[1:], " ")
		allowed := a.modelManager.GetAllowed()
		
		var selectedModel string
		
		// Try to parse as number first
		if num := parseModelNumber(selector); num > 0 && num <= len(allowed) {
			selectedModel = allowed[num-1]
		} else {
			// Try fuzzy match by partial name
			matches := findModelsByPartialName(allowed, selector)
			
			if len(matches) == 0 {
				return fmt.Sprintf("❌ No models match '%s'\n\nUse `/model list` to see available models.", selector), nil
			}
			
			if len(matches) > 1 {
				var response strings.Builder
				response.WriteString(fmt.Sprintf("❌ Multiple models match '%s':\n\n", selector))
				for i, model := range matches {
					response.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, model))
				}
				response.WriteString("\nPlease be more specific or use the model number.")
				return response.String(), nil
			}
			
			selectedModel = matches[0]
		}
		
		if err := a.modelManager.SetCurrent(selectedModel); err != nil {
			return fmt.Sprintf("❌ Error: %v\n\nUse `/model list` to see available models.", err), nil
		}
		
		// Update the LLM provider with the new model
		if openRouter, ok := a.llm.(*llm.OpenRouter); ok {
			openRouter.SetModel(selectedModel)
		}
		
		logger.Info("Model switched to: %s", selectedModel)
		return fmt.Sprintf("✅ Model switched to `%s`", selectedModel), nil
	}
	
	return "❌ Invalid command. Use `/model list` or `/model <number|name>`", nil
}

// parseModelNumber tries to parse a string as a positive integer
func parseModelNumber(s string) int {
	var num int
	if _, err := fmt.Sscanf(s, "%d", &num); err == nil && num > 0 {
		return num
	}
	return 0
}

// findModelsByPartialName finds models that contain the search string (case-insensitive)
func findModelsByPartialName(models []string, search string) []string {
	search = strings.ToLower(search)
	var matches []string
	
	for _, model := range models {
		if strings.Contains(strings.ToLower(model), search) {
			matches = append(matches, model)
		}
	}
	
	return matches
}
