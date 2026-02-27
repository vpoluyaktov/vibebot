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
			"**Conversation:**\n" +
			"/new - Start a new conversation (clears context)\n" +
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

	// Build system message
	systemMessage := systemPrompt
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
		logger.Debug("LLM response: content_len=%d, tool_calls=%d, finish_reason=%s", len(response.Content), len(response.ToolCalls), response.FinishReason)

		// Clean response content from XML artifacts (some models output <tool_call> tags)
		if response.Content != "" {
			response.Content = cleanXMLArtifacts(response.Content)
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

// cleanXMLArtifacts removes XML-style tool call tags that some models incorrectly output
func cleanXMLArtifacts(content string) string {
	// Remove <tool_call>...</tool_call> tags and their content
	// This handles cases where models output XML instead of using proper JSON tool calls
	content = strings.ReplaceAll(content, "<tool_call>", "")
	content = strings.ReplaceAll(content, "</tool_call>", "")
	
	// Also clean up any stray JSON that might be in the content
	// (some models put JSON outside of proper tool_calls structure)
	if strings.Contains(content, `{"name":`) && strings.Contains(content, `"arguments":`) {
		// If content looks like it contains tool call JSON, strip it
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
