package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/vpoluyaktov/vibebot/internal/consolidation"
	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
	"github.com/vpoluyaktov/vibebot/internal/memory"
	"github.com/vpoluyaktov/vibebot/internal/modelmanager"
	"github.com/vpoluyaktov/vibebot/internal/session"
	"github.com/vpoluyaktov/vibebot/internal/tools"
)

// ProgressCallback is called to send progress updates during task execution
type ProgressCallback func(chatID int64, message string, isToolHint bool)

const maxToolIterations = 40       // Prevent infinite loops (matches nanobot default)
const maxHistoryMessages = 50      // Maximum messages to include in context
const consolidationThreshold = 100 // Trigger consolidation when session exceeds this
const consolidationBatchSize = 50  // How many old messages to consolidate at once

// Agent coordinates the AI assistant behavior
type Agent struct {
	llm              llm.Provider
	memory           *memory.Memory
	tools            *tools.Registry
	sessions         *session.Manager
	modelManager     *modelmanager.Manager
	consolidator     *consolidation.Consolidator
	progressCallback ProgressCallback
	workspacePath    string
}

// New creates a new Agent instance
func New(provider llm.Provider, mem *memory.Memory, toolRegistry *tools.Registry, sessionMgr *session.Manager, modelMgr *modelmanager.Manager, workspacePath string) *Agent {
	return &Agent{
		llm:              provider,
		memory:           mem,
		tools:            toolRegistry,
		sessions:         sessionMgr,
		modelManager:     modelMgr,
		consolidator:     consolidation.New(provider, mem),
		progressCallback: nil,
		workspacePath:    workspacePath,
	}
}

// SetProgressCallback sets the callback for progress updates
func (a *Agent) SetProgressCallback(callback ProgressCallback) {
	a.progressCallback = callback
}

// ProcessMessage handles an incoming message and generates a response
func (a *Agent) ProcessMessage(ctx context.Context, chatID int64, message string) (string, error) {
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
			"**Conversation:**\n" +
			"/new - Start a new conversation (clears context)\n" +
			"/stop - Stop processing and clear queue\n" +
			"/help - Show this help message\n\n" +
			"**Model Management:**\n" +
			"/model, /models - Show current LLM model\n" +
			"/model list - List all available models\n" +
			"/model <number> - Switch by number\n" +
			"/model <partial-name> - Switch by name\n\n" +
			"**Project Management:**\n" +
			"/projects - List all projects\n" +
			"/project create <name> - Create new project\n" +
			"/project <name> - Switch to project\n" +
			"/project delete <name> - Delete project\n" +
			"/project clear - Clear current project\n\n" +
			"Just send me a message and I'll help you!", nil
	}

	// Handle /model and /models commands
	if strings.HasPrefix(message, "/model") || strings.HasPrefix(message, "/models") {
		return a.handleModelCommand(message)
	}

	// Handle /projects command
	if message == "/projects" {
		logger.Debug("Handling /projects command directly (no LLM)")
		return a.handleProjectsCommand(chatID)
	}

	// Handle /project command
	if strings.HasPrefix(message, "/project") {
		logger.Debug("Handling /project command directly (no LLM)")
		return a.handleProjectCommand(chatID, message)
	}

	// Get or create session for this chat
	sess := a.sessions.GetOrCreate(chatID)

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
		// Track token usage
		totalPromptTokens += response.Usage.PromptTokens
		totalCompletionTokens += response.Usage.CompletionTokens
		totalTokens += response.Usage.TotalTokens

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

		// Don't save tool call messages to session - they're implementation details

		// Execute each tool call
		for _, toolCall := range response.ToolCalls {
			// Log tool call with arguments (truncate if too long)
			args := toolCall.Function.Arguments
			if len(args) > 200 {
				args = args[:200] + "..."
			}
			logger.Debug("Executing tool: %s with args: %s", toolCall.Function.Name, args)

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

	// Check if consolidation is needed (run in background)
	if sess.NeedsConsolidation(consolidationThreshold) {
		sessionKey := fmt.Sprintf("telegram:%d", chatID)
		logger.Info("Session %s needs consolidation (%d messages)", sessionKey, len(sess.Messages))
		go a.consolidateSession(sessionKey)
	}

	// Append token usage and context window stats to final response
	contextSize := len(sess.GetHistory(maxHistoryMessages))

	// Build stats string with token usage and context percentage
	var tokenStats string
	contextLength := a.modelManager.GetContextLength()
	if contextLength > 0 {
		// Calculate percentage of context window used
		percentage := float64(totalPromptTokens) / float64(contextLength) * 100
		tokenStats = fmt.Sprintf("\n📊 Tokens: %s prompt + %s completion = %s total\n| Context: %.1f%%\n| History: %d/%d msgs",
			formatNumber(totalPromptTokens),
			formatNumber(totalCompletionTokens),
			formatNumber(totalTokens),
			percentage,
			contextSize,
			maxHistoryMessages)
	} else {
		// Fallback if context length not available
		tokenStats = fmt.Sprintf("\n📊 Tokens: %s prompt + %s completion = %s total\n| History: %d/%d msgs",
			formatNumber(totalPromptTokens),
			formatNumber(totalCompletionTokens),
			formatNumber(totalTokens),
			contextSize,
			maxHistoryMessages)
	}
	finalResponse += tokenStats

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
func (a *Agent) consolidateSession(sessionKey string) {
	ctx := context.Background()

	sess, err := a.sessions.GetSession(sessionKey)
	if err != nil {
		logger.Error("Failed to get session for consolidation: %v", err)
		return
	}

	// Get messages to consolidate
	messages := sess.GetMessagesForConsolidation(consolidationBatchSize)
	if len(messages) == 0 {
		logger.Debug("No messages to consolidate for session %s", sessionKey)
		return
	}

	logger.Info("Consolidating %d messages for session %s", len(messages), sessionKey)

	// Run consolidation
	result, err := a.consolidator.ConsolidateMessages(ctx, messages, sess.GetProject())
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

	if sess.GetProject() != "" && len(result.ProjectFacts) > 0 {
		logger.Info("Appending %d project facts to project '%s'", len(result.ProjectFacts), sess.GetProject())
		if err := a.memory.AppendProjectFacts(sess.GetProject(), result.ProjectFacts); err != nil {
			logger.Error("Failed to append project facts: %v", err)
		}
	}

	// Append summary to HISTORY.md
	if err := a.memory.AppendConsolidationSummary(result.Summary, sess.GetProject()); err != nil {
		logger.Error("Failed to append consolidation summary: %v", err)
	}

	// Mark messages as consolidated
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
				response.WriteString(fmt.Sprintf("%d. `%s` ✅\n", i+1, model))
			} else {
				response.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, model))
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

// handleProjectsCommand lists all available projects
func (a *Agent) handleProjectsCommand(chatID int64) (string, error) {
	// Get current project from session
	sess := a.sessions.GetOrCreate(chatID)
	currentProject := sess.GetProject()

	// List all projects using memory manager
	projects, err := a.memory.ListProjects()
	if err != nil {
		return fmt.Sprintf("❌ Error reading projects: %v", err), nil
	}

	if len(projects) == 0 {
		return "📂 **Projects**\n\nNo projects found. Use `/project create <name>` to create one.", nil
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("📂 **Projects** (%d total)\n\n", len(projects)))

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
	return response.String(), nil
}

// handleProjectCommand manages project creation, switching, and deletion
func (a *Agent) handleProjectCommand(chatID int64, message string) (string, error) {
	parts := strings.Fields(message)

	if len(parts) < 2 {
		return "❌ Usage:\n" +
			"- `/project create <name>` - Create a new project\n" +
			"- `/project <name>` - Switch to a project\n" +
			"- `/project delete <name>` - Delete a project\n" +
			"- `/project clear` - Clear current project\n" +
			"- `/projects` - List all projects", nil
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

		// Create project using memory manager
		if err := a.memory.CreateProject(projectName); err != nil {
			return fmt.Sprintf("❌ Error creating project: %v", err), nil
		}

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

	// Load project memory to verify it's readable
	projectMemory, err := a.memory.LoadProjectMemory(projectName)
	if err != nil {
		return fmt.Sprintf("❌ Error loading project: %v", err), nil
	}

	// Switch to project
	sess.SetProject(projectName)
	if err := a.sessions.Save(sess); err != nil {
		logger.Warn("Failed to save session after switching project: %v", err)
	}

	logger.Info("Switched to project: %s (chat %d)", projectName, chatID)
	return fmt.Sprintf("✅ Switched to project `%s`\n\nProject memory loaded (%d bytes). All context is now project-specific.", projectName, len(projectMemory)), nil
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

// formatToolHint formats tool calls as concise hints like: read_file("config.py"), exec("ls -la")
func formatToolHint(toolCalls []llm.ToolCall) string {
	if len(toolCalls) == 0 {
		return ""
	}

	hints := make([]string, 0, len(toolCalls))
	for _, tc := range toolCalls {
		// Arguments is a JSON string, try to extract first value for display
		var firstArg string
		if tc.Function.Arguments != "" {
			// Simple extraction: look for first string value in JSON
			// This is a best-effort display, not full parsing
			start := strings.Index(tc.Function.Arguments, `":"`)
			if start != -1 {
				start += 3 // Skip past ":"
				end := strings.Index(tc.Function.Arguments[start:], `"`)
				if end != -1 {
					firstArg = tc.Function.Arguments[start : start+end]
				}
			}
		}

		// Truncate long arguments
		if len(firstArg) > 40 {
			firstArg = firstArg[:40] + "..."
		}

		// Format as function call
		if firstArg != "" {
			hints = append(hints, fmt.Sprintf(`%s("%s")`, tc.Function.Name, firstArg))
		} else {
			hints = append(hints, tc.Function.Name)
		}
	}

	return "🔧 " + strings.Join(hints, ", ")
}
