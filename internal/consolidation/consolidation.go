package consolidation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
	"github.com/vpoluyaktov/vibebot/internal/memory"
)

// Consolidator handles session consolidation
type Consolidator struct {
	llmProvider llm.Provider
	memory      *memory.Memory
}

// New creates a new Consolidator
func New(llmProvider llm.Provider, mem *memory.Memory) *Consolidator {
	return &Consolidator{
		llmProvider: llmProvider,
		memory:      mem,
	}
}

// ConsolidationResult contains the output of consolidation
type ConsolidationResult struct {
	Summary            string   // Human-readable summary for HISTORY.md
	GlobalFacts        []string // Facts to add to GlobalMemory.md
	ProjectFacts       []string // Facts to add to project memory (if project active)
	ProjectDescription string   // Brief project description (for Description section)
	ProjectStatus      string   // Project status (for Status section)
	ProjectFocus       string   // Current focus (for Current Focus section)
	Timestamp          time.Time
}

// ConsolidateMessages summarizes old messages and extracts facts
func (c *Consolidator) ConsolidateMessages(
	ctx context.Context,
	messages []llm.Message,
	projectName string,
) (*ConsolidationResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages to consolidate")
	}

	logger.Info("Consolidating %d messages (project: %s)", len(messages), projectName)

	// Build consolidation prompt
	prompt := c.buildConsolidationPrompt(messages, projectName)

	// Call LLM
	systemPrompt := `You are a memory consolidation assistant. Your task is to analyze conversation history and extract important information.

Follow the output format exactly as specified. Be concise and extract only truly important information that should be remembered long-term.`

	response, err := c.llmProvider.Chat(ctx, []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}, nil) // No tools needed for consolidation

	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse response
	result, err := c.parseConsolidationResponse(response.Content, projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse consolidation response: %w", err)
	}

	result.Timestamp = time.Now()
	logger.Info("Consolidation complete: %d global facts, %d project facts",
		len(result.GlobalFacts), len(result.ProjectFacts))

	return result, nil
}

// buildConsolidationPrompt creates the prompt for LLM consolidation
func (c *Consolidator) buildConsolidationPrompt(messages []llm.Message, projectName string) string {
	var sb strings.Builder

	sb.WriteString("Please consolidate the following conversation messages:\n\n")
	sb.WriteString("## Messages to Consolidate\n\n")

	for i, msg := range messages {
		// Skip system messages and tool calls
		if msg.Role == "system" || msg.Role == "tool" {
			continue
		}

		sb.WriteString(fmt.Sprintf("**Message %d** (%s):\n", i+1, msg.Role))
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Current Project\n\n")
	if projectName != "" {
		sb.WriteString(fmt.Sprintf("Project: **%s**\n\n", projectName))
	} else {
		sb.WriteString("No active project\n\n")
	}

	sb.WriteString("## Your Task\n\n")
	sb.WriteString("1. Summarize the conversation in 2-3 paragraphs\n")
	sb.WriteString("2. Extract important facts that should be remembered long-term\n")
	sb.WriteString("3. Categorize facts as either:\n")
	sb.WriteString("   - **GLOBAL**: ONLY user preferences, timezone, name, cross-cutting concerns\n")
	sb.WriteString("   - **PROJECT**: Everything specific to the current project\n\n")
	sb.WriteString("**CRITICAL RULES FOR FACT CATEGORIZATION:**\n")
	sb.WriteString("- If a fact mentions the project name, code location, architecture, APIs, dependencies → PROJECT\n")
	sb.WriteString("- If a fact is about how THIS specific codebase works → PROJECT\n")
	sb.WriteString("- If a fact would NOT apply to other projects → PROJECT\n")
	sb.WriteString("- GLOBAL facts are RARE - only user info and universal preferences\n\n")
	sb.WriteString("**Examples of GLOBAL facts:**\n")
	sb.WriteString("- User's name is Vladimir\n")
	sb.WriteString("- User prefers Pacific timezone\n")
	sb.WriteString("- User prefers automatic task resumption\n\n")
	sb.WriteString("**Examples of PROJECT facts (NOT global):**\n")
	sb.WriteString("- Code location: /mnt/hostgit/projectname\n")
	sb.WriteString("- Uses OpenRouter API for LLM calls\n")
	sb.WriteString("- Session architecture stores 50 messages\n")
	sb.WriteString("- Written in Go with systemd service\n")
	sb.WriteString("- Any technical implementation details\n\n")
	sb.WriteString("**For project analysis conversations**, extract comprehensive information:\n")
	sb.WriteString("- Project purpose/description (what it does, main goals)\n")
	sb.WriteString("- Technical stack (languages, frameworks, key dependencies)\n")
	sb.WriteString("- Architecture (structure, components, design patterns)\n")
	sb.WriteString("- Key features and capabilities\n")
	sb.WriteString("- Current status (production-ready, in-development, etc.)\n")
	sb.WriteString("- Important technical decisions or constraints\n\n")

	sb.WriteString("## Output Format\n\n")
	sb.WriteString("Provide your response in the following format:\n\n")
	sb.WriteString("### SUMMARY\n")
	sb.WriteString("[2-3 paragraph summary of the conversation]\n\n")
	sb.WriteString("### PROJECT_DESCRIPTION\n")
	sb.WriteString("[1-2 sentence description of what the project does - only for project analysis conversations]\n\n")
	sb.WriteString("### PROJECT_STATUS\n")
	sb.WriteString("[Project status: Production-ready / In Development / Prototype / etc. - only for project analysis]\n\n")
	sb.WriteString("### PROJECT_FOCUS\n")
	sb.WriteString("[What the project is currently focused on - only for project analysis]\n\n")
	sb.WriteString("### GLOBAL_FACTS\n")
	sb.WriteString("- [Fact 1]\n")
	sb.WriteString("- [Fact 2]\n")
	sb.WriteString("...\n\n")
	sb.WriteString("### PROJECT_FACTS\n")
	sb.WriteString("- [Fact 1]\n")
	sb.WriteString("- [Fact 2]\n")
	sb.WriteString("...\n\n")
	sb.WriteString("If there are no facts in a category, write \"None\" under that section.\n")
	sb.WriteString("For non-project-analysis conversations, leave PROJECT_DESCRIPTION, PROJECT_STATUS, and PROJECT_FOCUS as \"None\".\n")

	return sb.String()
}

// parseConsolidationResponse parses the LLM response into structured result
func (c *Consolidator) parseConsolidationResponse(content string, projectName string) (*ConsolidationResult, error) {
	result := &ConsolidationResult{
		GlobalFacts:  []string{},
		ProjectFacts: []string{},
	}

	// Split into sections
	sections := strings.Split(content, "###")

	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}

		lines := strings.Split(section, "\n")
		if len(lines) == 0 {
			continue
		}

		header := strings.TrimSpace(lines[0])
		body := strings.Join(lines[1:], "\n")

		switch {
		case strings.Contains(strings.ToUpper(header), "SUMMARY"):
			result.Summary = strings.TrimSpace(body)

		case strings.Contains(strings.ToUpper(header), "PROJECT_DESCRIPTION"):
			if projectName != "" {
				desc := strings.TrimSpace(body)
				if desc != "" && strings.ToLower(desc) != "none" {
					result.ProjectDescription = desc
				}
			}

		case strings.Contains(strings.ToUpper(header), "PROJECT_STATUS"):
			if projectName != "" {
				status := strings.TrimSpace(body)
				if status != "" && strings.ToLower(status) != "none" {
					result.ProjectStatus = status
				}
			}

		case strings.Contains(strings.ToUpper(header), "PROJECT_FOCUS"):
			if projectName != "" {
				focus := strings.TrimSpace(body)
				if focus != "" && strings.ToLower(focus) != "none" {
					result.ProjectFocus = focus
				}
			}

		case strings.Contains(strings.ToUpper(header), "GLOBAL_FACTS"):
			result.GlobalFacts = c.extractFacts(body)

		case strings.Contains(strings.ToUpper(header), "PROJECT_FACTS"):
			if projectName != "" {
				result.ProjectFacts = c.extractFacts(body)
			}
		}
	}

	// Validate we got at least a summary
	if result.Summary == "" {
		return nil, fmt.Errorf("no summary found in consolidation response")
	}

	return result, nil
}

// extractFacts parses bullet points from markdown text
func (c *Consolidator) extractFacts(text string) []string {
	facts := []string{}
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and "None" markers
		if line == "" || strings.ToLower(line) == "none" {
			continue
		}

		// Extract bullet points (-, *, •)
		if strings.HasPrefix(line, "- ") {
			fact := strings.TrimPrefix(line, "- ")
			facts = append(facts, strings.TrimSpace(fact))
		} else if strings.HasPrefix(line, "* ") {
			fact := strings.TrimPrefix(line, "* ")
			facts = append(facts, strings.TrimSpace(fact))
		} else if strings.HasPrefix(line, "• ") {
			fact := strings.TrimPrefix(line, "• ")
			facts = append(facts, strings.TrimSpace(fact))
		}
	}

	return facts
}
