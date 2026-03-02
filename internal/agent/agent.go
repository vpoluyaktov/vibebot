package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vpoluyaktov/vibebot/internal/tasks"

	"github.com/vpoluyaktov/vibebot/internal/consolidation"
	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
	"github.com/vpoluyaktov/vibebot/internal/memory"
	"github.com/vpoluyaktov/vibebot/internal/modelmanager"
	"github.com/vpoluyaktov/vibebot/internal/session"
	"github.com/vpoluyaktov/vibebot/internal/telegram"
	"github.com/vpoluyaktov/vibebot/internal/tools"
)

// ProgressCallback is called to send progress updates during task execution
type ProgressCallback func(chatID int64, message string, isToolHint bool)

const maxToolIterations = 40       // Prevent infinite loops (matches nanobot default)
const maxHistoryMessages = 50      // Maximum messages to include in context
const consolidationThreshold = 100 // Trigger consolidation when session exceeds this
const consolidationBatchSize = 50  // How many old messages to consolidate at once

// KeyboardSender is an interface for sending messages with inline keyboards
type KeyboardSender interface {
	SendMessageWithKeyboard(chatID int64, text string, keyboard [][]telegram.InlineButton) error
}

// Agent coordinates the AI assistant behavior
// Agent coordinates the AI assistant behavior
type Agent struct {
	llm              llm.Provider
	memory           *memory.Memory
	tools            *tools.Registry
	sessions         *session.Manager
	modelManager     *modelmanager.Manager
	consolidator     *consolidation.Consolidator
	progressCallback ProgressCallback
	keyboardSender   KeyboardSender
	workspacePath    string
	taskManager      *tasks.Manager
}

// New creates a new Agent instance
func New(provider llm.Provider, mem *memory.Memory, toolRegistry *tools.Registry, sessionMgr *session.Manager, modelMgr *modelmanager.Manager, workspacePath string, taskMgr *tasks.Manager) *Agent {
	return &Agent{
		llm:              provider,
		memory:           mem,
		tools:            toolRegistry,
		sessions:         sessionMgr,
		modelManager:     modelMgr,
		consolidator:     consolidation.New(provider, mem),
		progressCallback: nil,
		workspacePath:    workspacePath,
		taskManager:      taskMgr,
	}
}

// SetProgressCallback sets the callback for progress updates
func (a *Agent) SetProgressCallback(callback ProgressCallback) {
	a.progressCallback = callback
}

// SetKeyboardSender sets the keyboard sender for inline keyboards
func (a *Agent) SetKeyboardSender(sender KeyboardSender) {
	a.keyboardSender = sender
}

// ProcessMessage handles an incoming message and generates a response
func (a *Agent) ProcessMessage(ctx context.Context, chatID int64, message string) (string, error) {
	// Add chat_id to context for tools
	ctx = context.WithValue(ctx, "chat_id", chatID)

	// Handle commands
	if message == "/new" {
		sess := a.sessions.GetOrCreate(chatID)

		// Trigger consolidation in background before clearing
		if len(sess.Messages) > 0 {
			sessionKey := fmt.Sprintf("telegram:%d", chatID)
			messages := make([]llm.Message, len(sess.Messages))
			copy(messages, sess.Messages)
			projectName := sess.GetProject()
			logger.Info("Triggering consolidation before /new (chat %d, %d messages)", chatID, len(messages))
			go a.consolidateSession(sessionKey, messages, projectName)
		}

		sess.Clear()
		if err := a.sessions.Save(sess); err != nil {
			logger.Warn("Failed to save cleared session: %v", err)
		}
		return " New conversation started. Previous context cleared.", nil
	}

	if message == "/help" {
		return "🤖 **vibebot** - AI Assistant\n\n" +
			"**Conversation:**\n" +
			"/new - Start a new conversation (clears context)\n" +
			"/stop - Stop processing and clear queue\n" +
			"/help - Show this help message\n\n" +
			"**Model Management:**\n" +
			"/model - Show available models with buttons to switch\n" +
			"/model <number> - Switch by number\n" +
			"/model <partial-name> - Switch by name\n\n" +
			"**Project Management:**\n" +
			"/project - Show projects with buttons to switch/create/clear\n" +
			"/project create <name> - Create new project\n" +
			"/project <name> - Switch to project\n" +
			"/project delete <name> - Delete project\n" +
			"/project clear - Clear current project\n\n" +
			"**Display Options:**\n" +
			"/verbose - Toggle tool usage display (On/Off)\n" +
			"/stats - Toggle token stats display (On/Off)\n\n" +
			"Just send me a message and I'll help you!", nil
	}

	// Handle /model command
	if strings.HasPrefix(message, "/model") {
		return a.handleModelCommand(chatID, message)
	}

	// Handle /project command
	if strings.HasPrefix(message, "/project") {
		logger.Debug("Handling /project command directly (no LLM)")
		return a.handleProjectCommand(chatID, message)
	}

	// Handle /verbose command
	if message == "/verbose" {
		return a.handleVerboseCommand(chatID)
	}

	// Handle /stats command
	if message == "/stats" {
		return a.handleStatsCommand(chatID)
	}

	// Get or create session for this chat
	sess := a.sessions.GetOrCreate(chatID)

	// Fetch current credits before processing (for diff calculation)
	var previousCredits float64
	if openRouter, ok := a.llm.(*llm.OpenRouter); ok {
		if credits, err := openRouter.FetchCredits(ctx); err == nil {
			previousCredits = credits
			logger.Debug("Current credit balance before processing: $%.4f", previousCredits)
		} else {
			// Use last known credits if fetch fails
			previousCredits = sess.GetLastCredits()
			logger.Warn("Failed to fetch current credits, using last known: $%.4f (error: %v)", previousCredits, err)
		}
	}

	// Add user message to session
	sess.AddMessage(llm.Message{
		Role:    "user",
		Content: message,
	})

	// Load global memory context
	memoryContent, err := a.memory.LoadMemory()
	if err != nil {
		logger.Warn("Failed to load memory: %v", err)
		memoryContent = ""
	}

	// Load project memory if a project is active
	currentProject := sess.GetProject()
	var projectMemory string
	if currentProject != "" {
		projectMemory, err = a.memory.LoadProjectMemory(currentProject)
		if err != nil {
			logger.Warn("Failed to load project memory for '%s': %v", currentProject, err)
			// Clear invalid project from session
			sess.ClearProject()
			if saveErr := a.sessions.Save(sess); saveErr != nil {
				logger.Warn("Failed to save session after clearing invalid project: %v", saveErr)
			}
			projectMemory = ""
			currentProject = ""
		} else {
			logger.Debug("Loaded project memory for '%s' (%d bytes)", currentProject, len(projectMemory))
		}
	}

	// Build system message with dynamic runtime info
	systemMessage := buildSystemPrompt(a.workspacePath)
	if memoryContent != "" {
		systemMessage += "\n\n## Global Memory (GlobalMemory.md)\n\n" + memoryContent
	}
	if currentProject != "" && projectMemory != "" {
		systemMessage += fmt.Sprintf("\n\n## Current Project: %s\n\n%s", currentProject, projectMemory)
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
	var totalPromptTokens, totalCompletionTokens, totalTokens int
	var totalCredits float64
	var toolsUsed []string
	for i := 0; i < maxToolIterations; i++ {
		logger.Debug("Agent loop iteration %d/%d", i+1, maxToolIterations)

		// Call LLM with available tools
		// Note: LLM provider returns errors as content (not Go errors) for graceful handling
		logger.Debug("Calling LLM with %d messages and %d tools", len(messages), len(a.tools.GetDefinitions()))
		response, err := a.llm.Chat(ctx, messages, a.tools.GetDefinitions())
		if err != nil {
			// This should rarely happen now since provider returns errors as content
			logger.Error("Unexpected LLM error (iteration %d): %v", i+1, err)
			return fmt.Sprintf("⚠️ Unexpected error: %v", err), err
		}
		// Track token usage and credits
		totalPromptTokens += response.Usage.PromptTokens
		totalCompletionTokens += response.Usage.CompletionTokens
		totalTokens += response.Usage.TotalTokens
		totalCredits += response.Usage.Credits

		logger.Debug("LLM response: content_len=%d, tool_calls=%d, finish_reason=%s, tokens=%d", len(response.Content), len(response.ToolCalls), response.FinishReason, response.Usage.TotalTokens)

		// Clean response content from XML artifacts (some models output <tool_call> tags)
		if response.Content != "" {
			response.Content = cleanXMLArtifacts(response.Content)
		}

		// If no tool calls, we're done
		if len(response.ToolCalls) == 0 {
			finalResponse = response.Content
			break
		}

		// Send progress updates (reasoning text only, no tool hints)
		if a.progressCallback != nil && response.Content != "" {
			a.progressCallback(chatID, response.Content, false)
		}

		// Add assistant message with tool calls to LLM context (for conversation loop only)
		assistantMsg := llm.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		}
		messages = append(messages, assistantMsg)

		// Save assistant message with tool calls to session
		sess.AddMessage(assistantMsg)

		// Execute each tool call
		for _, toolCall := range response.ToolCalls {
			// Log tool call with arguments (truncate if too long)
			args := toolCall.Function.Arguments
			if len(args) > 200 {
				args = args[:200] + "..."
			}
			logger.Debug("Executing tool: %s with args: %s", toolCall.Function.Name, args)

			// Track tool usage
			toolsUsed = append(toolsUsed, toolCall.Function.Name)

			result, err := a.tools.Execute(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
				logger.Error("Tool execution error for %s: %v", toolCall.Function.Name, err)
			} else {
				// Log successful execution with result summary
				resultSummary := result
				if len(result) > 150 {
					resultSummary = result[:150] + "..."
				}
				logger.Debug("Tool %s completed successfully: %s", toolCall.Function.Name, resultSummary)
			}

			// Add tool result to LLM context (for conversation loop only)
			toolMsg := llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
			}
			messages = append(messages, toolMsg)

			// Save tool result to session
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

	// Save session
	if err := a.sessions.Save(sess); err != nil {
		logger.Warn("Failed to save session: %v", err)
	}

	// Log conversation to history
	if err := a.memory.LogConversation(chatID, message, finalResponse); err != nil {
		logger.Warn("Failed to log conversation: %v", err)
	}

	// Check if consolidation is needed (run in background)
	if sess.NeedsConsolidation(consolidationThreshold) {
		sessionKey := fmt.Sprintf("telegram:%d", chatID)
		messages := make([]llm.Message, len(sess.Messages))
		copy(messages, sess.Messages)
		projectName := sess.GetProject()
		logger.Info("Session %s needs consolidation (%d messages)", sessionKey, len(messages))
		go a.consolidateSession(sessionKey, messages, projectName)
	}

	// Append verbose tool usage if enabled
	if sess.GetShowVerbose() && len(toolsUsed) > 0 {
		// Remove duplicates and format tool list
		uniqueTools := make(map[string]bool)
		for _, tool := range toolsUsed {
			uniqueTools[tool] = true
		}

		var toolList []string
		for tool := range uniqueTools {
			toolList = append(toolList, fmt.Sprintf("`%s`", tool))
		}

		finalResponse += fmt.Sprintf("\n🔧 Tools used: %s", strings.Join(toolList, ", "))
	}

	// Fetch current credits after processing (for diff calculation)
	var currentCredits float64
	var creditDiff float64
	var creditsAvailable bool
	if openRouter, ok := a.llm.(*llm.OpenRouter); ok {
		// Wait briefly for OpenRouter's balance to update
		time.Sleep(1 * time.Second)

		if credits, err := openRouter.FetchCredits(ctx); err == nil {
			currentCredits = credits
			creditDiff = previousCredits - currentCredits
			creditsAvailable = true
			sess.SetLastCredits(currentCredits)
			logger.Debug("Current credit balance after processing: $%.4f (diff: $%.6f)", currentCredits, creditDiff)
		} else {
			logger.Warn("Failed to fetch current credits after processing: %v", err)
			creditsAvailable = false
		}
	}

	// Append token usage and context window stats to final response if enabled
	if sess.GetShowStats() {
		contextSize := len(sess.GetHistory(maxHistoryMessages))

		// Build stats string with token usage, credits, and context percentage
		var tokenStats string
		contextLength := a.modelManager.GetContextLength()

		// Format credits display with diff tracking
		creditsStr := ""
		if creditsAvailable {
			// Display credit diff (cost of this request)
			if creditDiff >= 0.01 {
				creditsStr = fmt.Sprintf("\n| Credits used: $%.4f\n| Balance: $%.2f", creditDiff, currentCredits)
			} else if creditDiff > 0 {
				// For very small amounts, use more precision
				creditsStr = fmt.Sprintf("\n| Credits used: $%.6f\n| Balance: $%.2f", creditDiff, currentCredits)
			} else {
				// Zero cost (shouldn't happen, but handle it)
				creditsStr = fmt.Sprintf("\n| Credits used: $0.000000\n| Balance: $%.2f", currentCredits)
			}
		} else {
			// Fallback to totalCredits from X-OpenRouter-Usage header if available
			if totalCredits >= 0.01 {
				creditsStr = fmt.Sprintf("\n| Credits used: $%.4f", totalCredits)
			} else if totalCredits > 0 {
				creditsStr = fmt.Sprintf("\n| Credits used: $%.6f", totalCredits)
			} else {
				creditsStr = "\n| Credits used: $0.000000"
			}
		}

		if contextLength > 0 {
			percentage := float64(totalTokens) / float64(contextLength) * 100
			tokenStats = fmt.Sprintf("\n📊 Tokens: %s prompt + %s completion = %s total\n| Context: %.1f%%\n| History: %d/%d msgs%s",
				formatNumber(totalPromptTokens),
				formatNumber(totalCompletionTokens),
				formatNumber(totalTokens),
				percentage,
				contextSize,
				maxHistoryMessages,
				creditsStr)
		} else {
			// Fallback if context length not available
			tokenStats = fmt.Sprintf("\n📊 Tokens: %s prompt + %s completion = %s total\n| History: %d/%d msgs%s",
				formatNumber(totalPromptTokens),
				formatNumber(totalCompletionTokens),
				formatNumber(totalTokens),
				contextSize,
				maxHistoryMessages,
				creditsStr)
		}
		finalResponse += tokenStats
	}

	// Save session with updated credit balance
	if err := a.sessions.Save(sess); err != nil {
		logger.Warn("Failed to save session after updating credits: %v", err)
	}

	return finalResponse, nil
}

// formatNumber formats a number with thousand separators
func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, c)
	}
	return string(result)
}

// consolidateSession performs background consolidation of old messages
// Messages and project name are passed directly to avoid race conditions with session clearing
func (a *Agent) consolidateSession(sessionKey string, messages []llm.Message, projectName string) {
	ctx := context.Background()

	if len(messages) == 0 {
		logger.Debug("No messages to consolidate for session %s", sessionKey)
		return
	}

	logger.Info("Consolidating %d messages for session %s", len(messages), sessionKey)

	// Run consolidation
	result, err := a.consolidator.ConsolidateMessages(ctx, messages, projectName)
	if err != nil {
		logger.Error("Consolidation failed for session %s: %v", sessionKey, err)
		return
	}

	// Update memory files
	if len(result.GlobalFacts) > 0 {
		logger.Info("Appending %d global facts to GlobalMemory.md", len(result.GlobalFacts))
		if err := a.memory.AppendGlobalFacts(result.GlobalFacts); err != nil {
			logger.Error("Failed to append global facts: %v", err)
		}
	}

	if projectName != "" {
		// Update project template fields if consolidation extracted structured info
		if result.ProjectDescription != "" || result.ProjectStatus != "" || result.ProjectFocus != "" {
			logger.Info("Updating project '%s' template fields", projectName)
			if err := a.memory.UpdateProjectFields(projectName, result.ProjectDescription, result.ProjectStatus, result.ProjectFocus); err != nil {
				logger.Error("Failed to update project fields: %v", err)
			}
		}

		// Append project facts
		if len(result.ProjectFacts) > 0 {
			logger.Info("Appending %d project facts to project '%s'", len(result.ProjectFacts), projectName)
			if err := a.memory.AppendProjectFacts(projectName, result.ProjectFacts); err != nil {
				logger.Error("Failed to append project facts: %v", err)
			}
		}
	}

	// Append summary to HISTORY.md
	if err := a.memory.AppendConsolidationSummary(result.Summary, projectName); err != nil {
		logger.Error("Failed to append consolidation summary: %v", err)
	}

	// Mark messages as consolidated in session
	sess, err := a.sessions.GetSession(sessionKey)
	if err == nil {
		sess.MarkConsolidated(len(messages))

		// Optional: Prune consolidated messages to save space
		// Disabled for now - keeping full history
		// sess.PruneConsolidated()

		// Save updated session
		if err := a.sessions.SaveSession(sess); err != nil {
			logger.Error("Failed to save session after consolidation: %v", err)
			return
		}

		logger.Info("Consolidation complete for session %s (LastConsolidated: %d)", sessionKey, sess.LastConsolidated)
	}
}

// handleModelCommand processes /model commands
func (a *Agent) handleModelCommand(chatID int64, message string) (string, error) {
	parts := strings.Fields(message)

	// /model or /model list - show all models with inline keyboard
	if len(parts) == 1 || (len(parts) == 2 && parts[1] == "list") {
		current := a.modelManager.GetCurrent()
		allowed := a.modelManager.GetAllowed()

		// If keyboard sender is available, send inline keyboard
		if a.keyboardSender != nil {
			var response strings.Builder
			response.WriteString("🤖 **Available Models**\n\n")
			response.WriteString(fmt.Sprintf("Current: `%s`\n\n", current))
			response.WriteString("Select a model:")

			// Build inline keyboard (max 8 buttons per row for readability)
			var keyboard [][]telegram.InlineButton
			for i, model := range allowed {
				// Extract short name from model (e.g., "claude-3.5-sonnet" from full path)
				shortName := model
				if lastSlash := strings.LastIndex(model, "/"); lastSlash != -1 {
					shortName = model[lastSlash+1:]
				}

				// Add checkmark for current model
				buttonText := fmt.Sprintf("%d. %s", i+1, shortName)
				if model == current {
					buttonText += " ✅"
				}

				// Create callback data with model index
				button := telegram.InlineButton{
					Text:         buttonText,
					CallbackData: fmt.Sprintf("model:%d", i+1),
				}

				// Add button to keyboard (one per row for clarity)
				keyboard = append(keyboard, []telegram.InlineButton{button})
			}

			if err := a.keyboardSender.SendMessageWithKeyboard(chatID, response.String(), keyboard); err != nil {
				logger.Warn("Failed to send keyboard, falling back to text: %v", err)
				// Fall back to text-only response
				return a.handleModelCommandText(current, allowed), nil
			}
			return "", nil // Message already sent via keyboard
		}

		// Fallback: text-only response
		return a.handleModelCommandText(current, allowed), nil
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

// handleModelCommandText returns text-only model list (fallback)
func (a *Agent) handleModelCommandText(current string, allowed []string) string {
	var response strings.Builder
	response.WriteString("🤖 **Available Models**\n\n")

	for i, model := range allowed {
		if model == current {
			response.WriteString(fmt.Sprintf("%d. `%s` ✅\n", i+1, model))
		} else {
			response.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, model))
		}
	}

	response.WriteString("\n💡 Use `/model <number>` or `/model <partial-name>` to switch")
	return response.String()
}

// handleProjectCommand manages project creation, switching, and deletion
func (a *Agent) handleProjectCommand(chatID int64, message string) (string, error) {
	parts := strings.Fields(message)

	// /project without arguments - show project list with keyboard
	if len(parts) == 1 {
		return a.showProjectList(chatID)
	}

	sess := a.sessions.GetOrCreate(chatID)
	subcommand := parts[1]

	// Handle /project create <name>
	if subcommand == "create" {
		if len(parts) < 3 {
			return "❌ Usage: `/project create <name>`", nil
		}

		projectName := parts[2]

		// Validate project name
		if err := a.memory.ValidateProjectName(projectName); err != nil {
			return fmt.Sprintf("❌ Invalid project name: %v", err), nil
		}

		// Check if project already exists
		if a.memory.ProjectExists(projectName) {
			return fmt.Sprintf("❌ Project `%s` already exists!", projectName), nil
		}

		// Trigger consolidation in background before switching projects
		if len(sess.Messages) > 0 {
			sessionKey := fmt.Sprintf("telegram:%d", chatID)
			messages := make([]llm.Message, len(sess.Messages))
			copy(messages, sess.Messages)
			projectName := sess.GetProject()
			logger.Info("Triggering consolidation before creating project (chat %d, %d messages)", chatID, len(messages))
			go a.consolidateSession(sessionKey, messages, projectName)
		}

		// Create project using memory manager
		if err := a.memory.CreateProject(projectName); err != nil {
			return fmt.Sprintf("❌ Error creating project: %v", err), nil
		}

		// Clear session context to start fresh with new project
		sess.Clear()

		// Automatically switch to the new project
		sess.SetProject(projectName)
		if err := a.sessions.Save(sess); err != nil {
			logger.Warn("Failed to save session after creating project: %v", err)
		}

		logger.Info("Created and switched to project: %s", projectName)
		return fmt.Sprintf("✅ Project `%s` created and activated!\n\nYou can now add project-specific context. Use `/project clear` to return to global context.", projectName), nil
	}

	// Handle /project delete <name>
	if subcommand == "delete" {
		if len(parts) < 3 {
			return "❌ Usage: `/project delete <name>`", nil
		}

		projectName := parts[2]
		currentProject := sess.GetProject()

		// Prevent deleting current project
		if projectName == currentProject {
			return fmt.Sprintf("❌ Cannot delete active project `%s`!\n\nUse `/project clear` first, then delete.", projectName), nil
		}

		// Delete project using memory manager
		if err := a.memory.DeleteProject(projectName); err != nil {
			return fmt.Sprintf("❌ Error deleting project: %v", err), nil
		}

		logger.Info("Deleted project: %s", projectName)
		return fmt.Sprintf("✅ Project `%s` deleted!", projectName), nil
	}

	// Handle /project clear - clear current project
	if subcommand == "clear" {
		currentProject := sess.GetProject()
		if currentProject == "" {
			return "ℹ️ No active project. Already using global context only.", nil
		}

		sess.ClearProject()
		if err := a.sessions.Save(sess); err != nil {
			logger.Warn("Failed to save session after clearing project: %v", err)
		}

		logger.Info("Cleared project context for chat %d (was: %s)", chatID, currentProject)
		return fmt.Sprintf("✅ Cleared project `%s`\n\nNow using global context only.", currentProject), nil
	}

	// Handle /project <name> - switch to project
	projectName := subcommand

	// Validate project name
	if err := a.memory.ValidateProjectName(projectName); err != nil {
		return fmt.Sprintf("❌ Invalid project name: %v", err), nil
	}

	// Check if project exists
	if !a.memory.ProjectExists(projectName) {
		return fmt.Sprintf("❌ Project `%s` does not exist!\n\nUse `/projects` to see available projects or `/project create %s` to create it.", projectName, projectName), nil
	}

	// Trigger consolidation in background before switching projects
	if len(sess.Messages) > 0 {
		sessionKey := fmt.Sprintf("telegram:%d", chatID)
		messages := make([]llm.Message, len(sess.Messages))
		copy(messages, sess.Messages)
		currentProject := sess.GetProject()
		logger.Info("Triggering consolidation before switching to project '%s' (chat %d, %d messages)", projectName, chatID, len(messages))
		go a.consolidateSession(sessionKey, messages, currentProject)
	}

	// Load project memory to verify it's readable
	projectMemory, err := a.memory.LoadProjectMemory(projectName)
	if err != nil {
		return fmt.Sprintf("❌ Error loading project: %v", err), nil
	}

	// Clear session context to avoid mixing contexts between projects
	sess.Clear()

	// Switch to project
	sess.SetProject(projectName)
	if err := a.sessions.Save(sess); err != nil {
		logger.Warn("Failed to save session after switching project: %v", err)
	}

	logger.Info("Switched to project: %s (chat %d)", projectName, chatID)
	return fmt.Sprintf("✅ Switched to project `%s`\n\nProject memory loaded (%d bytes). All context is now project-specific.", projectName, len(projectMemory)), nil
}

// showProjectList displays project list with inline keyboard
func (a *Agent) showProjectList(chatID int64) (string, error) {
	// Get current project from session
	sess := a.sessions.GetOrCreate(chatID)
	currentProject := sess.GetProject()

	// List all projects using memory manager
	projects, err := a.memory.ListProjects()
	if err != nil {
		return fmt.Sprintf("❌ Error reading projects: %v", err), nil
	}

	// If keyboard sender is available, send inline keyboard
	if a.keyboardSender != nil {
		var response strings.Builder
		response.WriteString("📂 **Project Management**\n\n")
		if currentProject != "" {
			response.WriteString(fmt.Sprintf("Current: `%s`\n\n", currentProject))
		}
		if len(projects) > 0 {
			response.WriteString(fmt.Sprintf("Projects (%d):", len(projects)))
		} else {
			response.WriteString("No projects yet. Create one to get started!")
		}

		// Build inline keyboard
		var keyboard [][]telegram.InlineButton

		// Add project buttons
		for _, project := range projects {
			buttonText := project
			if project == currentProject {
				buttonText += " ✅"
			}

			button := telegram.InlineButton{
				Text:         buttonText,
				CallbackData: fmt.Sprintf("project:%s", project),
			}
			keyboard = append(keyboard, []telegram.InlineButton{button})
		}

		// Add "Create New Project" button
		keyboard = append(keyboard, []telegram.InlineButton{{
			Text:         "➕ Create New Project",
			CallbackData: "project:create",
		}})

		// Add "Delete Current Project" button if there's a current project
		if currentProject != "" {
			keyboard = append(keyboard, []telegram.InlineButton{{
				Text:         "❌ Delete Current Project",
				CallbackData: "project:delete",
			}})
		}

		if err := a.keyboardSender.SendMessageWithKeyboard(chatID, response.String(), keyboard); err != nil {
			logger.Warn("Failed to send keyboard, falling back to text: %v", err)
			// Fall back to text-only response
			return a.showProjectListText(currentProject, projects), nil
		}
		return "", nil // Message already sent via keyboard
	}

	// Fallback: text-only response
	return a.showProjectListText(currentProject, projects), nil
}

// showProjectListText returns text-only project list (fallback)
func (a *Agent) showProjectListText(currentProject string, projects []string) string {
	var response strings.Builder
	response.WriteString("📂 **Project Management**\n\n")

	if len(projects) == 0 {
		response.WriteString("No projects found. Use `/project create <name>` to create one.")
		return response.String()
	}

	response.WriteString(fmt.Sprintf("Projects (%d total):\n\n", len(projects)))

	for _, project := range projects {
		if project == currentProject {
			response.WriteString(fmt.Sprintf("✅ `%s` (current)\n", project))
		} else {
			response.WriteString(fmt.Sprintf("   `%s`\n", project))
		}
	}

	response.WriteString("\n💡 Use `/project <name>` to switch")
	if currentProject != "" {
		response.WriteString("\n💡 Use `/project clear` to clear current project")
	}
	response.WriteString("\n💡 Use `/project create <name>` to create new")
	return response.String()
}

// handleVerboseCommand toggles verbose tool usage display
func (a *Agent) handleVerboseCommand(chatID int64) (string, error) {
	sess := a.sessions.GetOrCreate(chatID)
	currentValue := sess.GetShowVerbose()
	newValue := !currentValue

	sess.SetShowVerbose(newValue)
	if err := a.sessions.Save(sess); err != nil {
		logger.Warn("Failed to save session after toggling verbose: %v", err)
	}

	status := "Off"
	emoji := "🔕"
	if newValue {
		status = "On"
		emoji = "🔔"
	}

	// Send response with inline keyboard
	if a.keyboardSender != nil {
		var keyboard [][]telegram.InlineButton

		// Create toggle button
		buttonText := fmt.Sprintf("Verbose: %s %s", status, emoji)
		button := telegram.InlineButton{
			Text:         buttonText,
			CallbackData: "verbose:toggle",
		}
		keyboard = append(keyboard, []telegram.InlineButton{button})

		message := fmt.Sprintf("%s **Verbose Mode: %s**\n\nTool usage will %sbe displayed after LLM responses.",
			emoji, status, map[bool]string{true: "", false: "not "}[newValue])

		if err := a.keyboardSender.SendMessageWithKeyboard(chatID, message, keyboard); err != nil {
			logger.Warn("Failed to send keyboard, falling back to text: %v", err)
			return message, nil
		}
		return "", nil
	}

	// Fallback: text-only response
	return fmt.Sprintf("%s **Verbose Mode: %s**\n\nTool usage will %sbe displayed after LLM responses.",
		emoji, status, map[bool]string{true: "", false: "not "}[newValue]), nil
}

// handleStatsCommand toggles token stats display
func (a *Agent) handleStatsCommand(chatID int64) (string, error) {
	sess := a.sessions.GetOrCreate(chatID)
	currentValue := sess.GetShowStats()
	newValue := !currentValue

	sess.SetShowStats(newValue)
	if err := a.sessions.Save(sess); err != nil {
		logger.Warn("Failed to save session after toggling stats: %v", err)
	}

	status := "Off"
	emoji := "🔕"
	if newValue {
		status = "On"
		emoji = "📊"
	}

	// Send response with inline keyboard
	if a.keyboardSender != nil {
		var keyboard [][]telegram.InlineButton

		// Create toggle button
		buttonText := fmt.Sprintf("Stats: %s %s", status, emoji)
		button := telegram.InlineButton{
			Text:         buttonText,
			CallbackData: "stats:toggle",
		}
		keyboard = append(keyboard, []telegram.InlineButton{button})

		message := fmt.Sprintf("%s **Token Stats: %s**\n\nToken usage statistics will %sbe displayed after LLM responses.",
			emoji, status, map[bool]string{true: "", false: "not "}[newValue])

		if err := a.keyboardSender.SendMessageWithKeyboard(chatID, message, keyboard); err != nil {
			logger.Warn("Failed to send keyboard, falling back to text: %v", err)
			return message, nil
		}
		return "", nil
	}

	// Fallback: text-only response
	return fmt.Sprintf("%s **Token Stats: %s**\n\nToken usage statistics will %sbe displayed after LLM responses.",
		emoji, status, map[bool]string{true: "", false: "not "}[newValue]), nil
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

// cleanXMLArtifacts removes XML-like artifacts that some models output
func cleanXMLArtifacts(content string) string {
	// Remove <tool_call> tags and similar artifacts
	re := regexp.MustCompile(`</?tool_call[^>]*>`)
	content = re.ReplaceAllString(content, "")

	// If content looks like it contains tool call JSON, strip it
	if strings.Contains(content, `{"name":`) && strings.Contains(content, `"arguments":`) {
		lines := strings.Split(content, "\n")
		var cleaned []string
		inJSON := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, `"name"`) {
				inJSON = true
				continue
			}
			if inJSON && strings.HasPrefix(trimmed, "}") {
				inJSON = false
				continue
			}
			if !inJSON {
				cleaned = append(cleaned, line)
			}
		}
		content = strings.Join(cleaned, "\n")
	}

	return strings.TrimSpace(content)
}

// ProcessCallback handles callback queries from inline keyboards
func (a *Agent) ProcessCallback(ctx context.Context, chatID int64, callbackData string) (string, error) {
	logger.Debug("Processing callback: %s", callbackData)

	// Parse callback data format: "command:value"
	parts := strings.SplitN(callbackData, ":", 2)
	if len(parts) != 2 {
		return "❌ Invalid callback data", nil
	}

	command := parts[0]
	value := parts[1]

	switch command {
	case "model":
		// Handle model selection: "model:1", "model:2", etc.
		return a.handleModelCallback(value)

	case "project":
		// Handle project selection: "project:myproject" or "project:clear"
		return a.handleProjectCallback(chatID, value)

	case "verbose":
		// Handle verbose toggle
		return a.handleVerboseCallback(chatID)

	case "stats":
		// Handle stats toggle
		return a.handleStatsCallback(chatID)

	default:
		return fmt.Sprintf("❌ Unknown callback command: %s", command), nil
	}
}

// handleModelCallback handles model selection from inline keyboard
func (a *Agent) handleModelCallback(value string) (string, error) {
	allowed := a.modelManager.GetAllowed()

	// Parse model number
	var modelNum int
	if _, err := fmt.Sscanf(value, "%d", &modelNum); err != nil || modelNum < 1 || modelNum > len(allowed) {
		return fmt.Sprintf("❌ Invalid model number: %s", value), nil
	}

	selectedModel := allowed[modelNum-1]

	if err := a.modelManager.SetCurrent(selectedModel); err != nil {
		return fmt.Sprintf("❌ Error: %v", err), nil
	}

	// Update the LLM provider with the new model
	if openRouter, ok := a.llm.(*llm.OpenRouter); ok {
		openRouter.SetModel(selectedModel)
	}

	logger.Info("Model switched to: %s (via callback)", selectedModel)
	return fmt.Sprintf("✅ Model switched to `%s`", selectedModel), nil
}

// handleProjectCallback handles project selection from inline keyboard
func (a *Agent) handleProjectCallback(chatID int64, value string) (string, error) {
	sess := a.sessions.GetOrCreate(chatID)

	// Handle "create" command - prompt for project name
	if value == "create" {
		return "➕ **Create New Project**\n\n" +
			"Please send the project name.\n\n" +
			"Example: `/project create myproject`\n\n" +
			"_Project names must be alphanumeric with optional hyphens/underscores._", nil
	}

	// Handle "delete" command - delete current project and switch to another
	if value == "delete" {
		currentProject := sess.GetProject()
		if currentProject == "" {
			return "ℹ️ No active project to delete.", nil
		}

		// Trigger consolidation in background before deleting
		if len(sess.Messages) > 0 {
			sessionKey := fmt.Sprintf("telegram:%d", chatID)
			messages := make([]llm.Message, len(sess.Messages))
			copy(messages, sess.Messages)
			logger.Info("Triggering consolidation before deleting project '%s' (chat %d, %d messages) via callback", currentProject, chatID, len(messages))
			go a.consolidateSession(sessionKey, messages, currentProject)
		}

		// Clear session context
		sess.Clear()

		// Delete the project
		if err := a.memory.DeleteProject(currentProject); err != nil {
			return fmt.Sprintf("❌ Error deleting project: %v", err), nil
		}

		// Get remaining projects
		projects, err := a.memory.ListProjects()
		if err != nil {
			logger.Warn("Failed to list projects after deletion: %v", err)
			projects = []string{}
		}

		// Auto-switch to first available project or clear to global context
		if len(projects) > 0 {
			newProject := projects[0]
			sess.SetProject(newProject)
			if err := a.sessions.Save(sess); err != nil {
				logger.Warn("Failed to save session after auto-switching project: %v", err)
			}
			logger.Info("Deleted project '%s' and switched to '%s' (chat %d) via callback", currentProject, newProject, chatID)
			return fmt.Sprintf("✅ Project `%s` deleted!\n\nSwitched to project `%s`.", currentProject, newProject), nil
		} else {
			sess.ClearProject()
			if err := a.sessions.Save(sess); err != nil {
				logger.Warn("Failed to save session after clearing project: %v", err)
			}
			logger.Info("Deleted project '%s' and switched to global context (chat %d) via callback", currentProject, chatID)
			return fmt.Sprintf("✅ Project `%s` deleted!\n\nNo other projects available. Using global context.", currentProject), nil
		}
	}

	// Handle project switch
	projectName := value

	// Validate project name
	if err := a.memory.ValidateProjectName(projectName); err != nil {
		return fmt.Sprintf("❌ Invalid project name: %v", err), nil
	}

	// Check if project exists
	if !a.memory.ProjectExists(projectName) {
		return fmt.Sprintf("❌ Project `%s` does not exist!", projectName), nil
	}

	// Trigger consolidation in background before switching projects
	if len(sess.Messages) > 0 {
		sessionKey := fmt.Sprintf("telegram:%d", chatID)
		messages := make([]llm.Message, len(sess.Messages))
		copy(messages, sess.Messages)
		currentProject := sess.GetProject()
		logger.Info("Triggering consolidation before switching to project '%s' (chat %d, %d messages) via callback", projectName, chatID, len(messages))
		go a.consolidateSession(sessionKey, messages, currentProject)
	}

	// Load project memory to verify it's readable
	projectMemory, err := a.memory.LoadProjectMemory(projectName)
	if err != nil {
		return fmt.Sprintf("❌ Error loading project: %v", err), nil
	}

	// Clear session context to avoid mixing contexts between projects
	sess.Clear()

	// Switch to project
	sess.SetProject(projectName)
	if err := a.sessions.Save(sess); err != nil {
		logger.Warn("Failed to save session after switching project: %v", err)
	}

	logger.Info("Switched to project: %s (chat %d) via callback", projectName, chatID)
	return fmt.Sprintf("✅ Switched to project `%s`\n\nProject memory loaded (%d bytes). All context is now project-specific.", projectName, len(projectMemory)), nil
}

// handleVerboseCallback handles verbose toggle from inline keyboard
func (a *Agent) handleVerboseCallback(chatID int64) (string, error) {
	sess := a.sessions.GetOrCreate(chatID)
	currentValue := sess.GetShowVerbose()
	newValue := !currentValue

	sess.SetShowVerbose(newValue)
	if err := a.sessions.Save(sess); err != nil {
		logger.Warn("Failed to save session after toggling verbose: %v", err)
	}

	status := "Off"
	emoji := "🔕"
	if newValue {
		status = "On"
		emoji = "🔔"
	}

	logger.Info("Verbose mode toggled to %s for chat %d", status, chatID)
	return fmt.Sprintf("%s **Verbose Mode: %s**\n\nTool usage will %sbe displayed after LLM responses.",
		emoji, status, map[bool]string{true: "", false: "not "}[newValue]), nil
}

// handleStatsCallback handles stats toggle from inline keyboard
func (a *Agent) handleStatsCallback(chatID int64) (string, error) {
	sess := a.sessions.GetOrCreate(chatID)
	currentValue := sess.GetShowStats()
	newValue := !currentValue

	sess.SetShowStats(newValue)
	if err := a.sessions.Save(sess); err != nil {
		logger.Warn("Failed to save session after toggling stats: %v", err)
	}

	status := "Off"
	emoji := "🔕"
	if newValue {
		status = "On"
		emoji = "📊"
	}

	logger.Info("Stats mode toggled to %s for chat %d", status, chatID)
	return fmt.Sprintf("%s **Token Stats: %s**\n\nToken usage statistics will %sbe displayed after LLM responses.",
		emoji, status, map[bool]string{true: "", false: "not "}[newValue]), nil
}

// ResumeTask resumes a task when timer expires
func (a *Agent) ResumeTask(ctx context.Context, chatID int64, taskContext string) error {
	logger.Info("Resuming task for chat %d: %s", chatID, taskContext)

	// Get session
	sess := a.sessions.GetOrCreate(chatID)

	// Add task context as a system message to guide the LLM
	systemMsg := fmt.Sprintf("A timer you set has expired. Task context: %s\n\nPlease check on this task and report the results to the user.", taskContext)

	// Add user message to session
	sess.AddMessage(llm.Message{
		Role:    "user",
		Content: systemMsg,
	})

	// Process the message
	response, err := a.ProcessMessage(ctx, chatID, systemMsg)
	if err != nil {
		logger.Error("Failed to process task resumption for chat %d: %v", chatID, err)
		return err
	}

	// Send notification to user
	if a.progressCallback != nil {
		a.progressCallback(chatID, response, false)
	}

	return nil
}
