package tools

import (
	"context"
	"fmt"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/memory"
)

// ConsolidateMemoryTool creates the consolidate_project_memory tool
func ConsolidateMemoryTool(consolidator *memory.Consolidator) *Tool {
	return &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "consolidate_project_memory",
				Description: "Consolidate and deduplicate project memory to reduce token usage. This analyzes all consolidated facts sections, removes duplicates, merges similar information, and creates a single optimized summary. Use this when project memory has grown too large (>15KB) or contains many redundant consolidated facts sections.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"project_name": map[string]interface{}{
							"type":        "string",
							"description": "Name of the project to consolidate (e.g., 'vibebot', 'my-project')",
						},
						"check_only": map[string]interface{}{
							"type":        "boolean",
							"description": "If true, only check if consolidation is needed without performing it. Returns statistics about the project memory.",
						},
					},
					"required": []string{"project_name"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			projectName, ok := args["project_name"].(string)
			if !ok {
				return "", fmt.Errorf("project_name must be a string")
			}

			checkOnly := false
			if val, ok := args["check_only"].(bool); ok {
				checkOnly = val
			}

			// Get statistics
			stats, err := consolidator.GetConsolidationStats(projectName)
			if err != nil {
				return "", fmt.Errorf("failed to get consolidation stats: %w", err)
			}

			if checkOnly {
				// Return statistics only
				return fmt.Sprintf(`Project Memory Statistics for '%s':
- Size: %d bytes (~%d tokens)
- Consolidated sections: %d
- Total facts: %d
- Needs consolidation: %v
- Threshold: 15,000 bytes

%s`,
					projectName,
					stats["size_bytes"],
					stats["estimated_tokens"],
					stats["consolidated_sections"],
					stats["total_facts"],
					stats["needs_consolidation"],
					func() string {
						if stats["needs_consolidation"].(bool) {
							return "✅ Consolidation recommended to reduce token usage"
						}
						return "✓ Size is within acceptable limits"
					}(),
				), nil
			}

			// Check if consolidation is needed
			needsConsolidation, err := consolidator.NeedsConsolidation(projectName)
			if err != nil {
				return "", fmt.Errorf("failed to check consolidation need: %w", err)
			}

			if !needsConsolidation {
				return fmt.Sprintf("Project '%s' does not need consolidation yet (%d bytes, threshold: 15,000 bytes)", 
					projectName, stats["size_bytes"]), nil
			}

			// Perform consolidation
			if err := consolidator.ConsolidateProject(ctx, projectName); err != nil {
				return "", fmt.Errorf("consolidation failed: %w", err)
			}

			// Get new statistics
			newStats, err := consolidator.GetConsolidationStats(projectName)
			if err != nil {
				return "", fmt.Errorf("failed to get post-consolidation stats: %w", err)
			}

			oldSize := stats["size_bytes"].(int)
			newSize := newStats["size_bytes"].(int)
			reduction := float64(oldSize-newSize) / float64(oldSize) * 100

			return fmt.Sprintf(`✅ Project memory consolidated successfully!

Project: %s
Before: %d bytes (~%d tokens)
After: %d bytes (~%d tokens)
Reduction: %.1f%%

Consolidated %d facts sections into a single optimized summary.
This will reduce prompt token usage on every request.`,
				projectName,
				oldSize,
				stats["estimated_tokens"],
				newSize,
				newStats["estimated_tokens"],
				reduction,
				stats["consolidated_sections"],
			), nil
		},
	}
}
