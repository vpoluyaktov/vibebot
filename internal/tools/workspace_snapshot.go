package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
)

func RegisterWorkspaceSnapshot(registry *Registry, workspaceDir string) {
	registry.Register("workspace_snapshot", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "workspace_snapshot",
				Description: "Get a snapshot of workspace structure and key files. Useful for understanding project layout without multiple list_dir calls.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"include_structure": map[string]interface{}{
							"type":        "boolean",
							"description": "Include directory tree (default: true)",
							"default":     true,
						},
						"include_summaries": map[string]interface{}{
							"type":        "boolean",
							"description": "Include key files like README, package.json, go.mod (default: true)",
							"default":     true,
						},
						"max_depth": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum directory depth (default: 3)",
							"default":     3,
						},
					},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			includeStructure := true
			if inc, ok := args["include_structure"].(bool); ok {
				includeStructure = inc
			}

			includeSummaries := true
			if inc, ok := args["include_summaries"].(bool); ok {
				includeSummaries = inc
			}

			maxDepth := 3
			if depth, ok := args["max_depth"].(float64); ok {
				maxDepth = int(depth)
			}

			var output strings.Builder
			output.WriteString(fmt.Sprintf("Workspace Snapshot: %s\n\n", workspaceDir))

			// Include directory structure
			if includeStructure {
				output.WriteString("## Directory Structure\n\n")
				structure := buildDirectoryTree(workspaceDir, maxDepth)
				output.WriteString(structure)
				output.WriteString("\n")
			}

			// Include key files
			if includeSummaries {
				output.WriteString("## Key Files\n\n")
				keyFiles := []string{"README.md", "README", "package.json", "go.mod", "requirements.txt", "Cargo.toml"}

				for _, filename := range keyFiles {
					path := filepath.Join(workspaceDir, filename)
					if data, err := os.ReadFile(path); err == nil {
						output.WriteString(fmt.Sprintf("### %s\n", filename))
						// Truncate long files
						content := string(data)
						if len(content) > 500 {
							content = content[:500] + "\n...(truncated)"
						}
						output.WriteString(content)
						output.WriteString("\n\n")
					}
				}
			}

			logger.Debug("workspace_snapshot: generated snapshot with depth %d", maxDepth)
			return output.String(), nil
		},
	})
}

func buildDirectoryTree(root string, maxDepth int) string {
	var builder strings.Builder
	buildTree(root, "", 0, maxDepth, &builder)
	return builder.String()
}

func buildTree(path string, prefix string, depth int, maxDepth int, builder *strings.Builder) {
	if depth > maxDepth {
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	for i, entry := range entries {
		// Skip hidden files and common ignore patterns
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" || entry.Name() == "vendor" {
			continue
		}

		isLast := i == len(entries)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		builder.WriteString(prefix + connector + entry.Name())
		if entry.IsDir() {
			builder.WriteString("/")
		}
		builder.WriteString("\n")

		if entry.IsDir() {
			newPrefix := prefix
			if isLast {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
			buildTree(filepath.Join(path, entry.Name()), newPrefix, depth+1, maxDepth, builder)
		}
	}
}
