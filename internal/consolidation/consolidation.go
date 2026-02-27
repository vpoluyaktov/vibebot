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
	Summary      string    // Human-readable summary for HISTORY.md
	GlobalFacts  []string  // Facts to add to GlobalMemory.md
	ProjectFacts []string  // Facts to add to project memory (if project active)
	Timestamp    time.Time
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
	sb.WriteString("   - GLOBAL: Cross-project facts (user preferences, general knowledge, system info)\n")
	sb.WriteString("   - PROJECT: Project-specific facts (only if project is active)\n\n")

	sb.WriteString("## Output Format\n\n")
	sb.WriteString("Provide your response in the following format:\n\n")
	sb.WriteString("### SUMMARY\n")
	sb.WriteString("[2-3 paragraph summary of the conversation]\n\n")
	sb.WriteString("### GLOBAL_FACTS\n")
	sb.WriteString("- [Fact 1]\n")
	sb.WriteString("- [Fact 2]\n")
	sb.WriteString("...\n\n")
	sb.WriteString("### PROJECT_FACTS\n")
	sb.WriteString("- [Fact 1]\n")
	sb.WriteString("- [Fact 2]\n")
	sb.WriteString("...\n\n")
	sb.WriteString("If there are no facts in a category, write \"None\" under that section.\n")

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
