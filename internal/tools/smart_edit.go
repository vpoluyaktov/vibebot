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

func RegisterSmartEdit(registry *Registry, workspaceDir string) {
	registry.Register("smart_edit", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "smart_edit",
				Description: "Context-aware editing for common patterns like adding imports, functions, etc. Handles the read-edit-verify cycle automatically.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "File path to edit",
						},
						"operation": map[string]interface{}{
							"type":        "string",
							"description": "Operation type: add_import, add_function, append_content",
							"enum":        []string{"add_import", "add_function", "append_content"},
						},
						"value": map[string]interface{}{
							"type":        "string",
							"description": "Value to add (import path, function code, content to append)",
						},
					},
					"required": []string{"path", "operation", "value"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			operation, _ := args["operation"].(string)
			value, _ := args["value"].(string)

			if path == "" || operation == "" || value == "" {
				return "", fmt.Errorf("path, operation, and value are required")
			}

			// Resolve path
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspaceDir, path)
			}

			// Read file
			data, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("failed to read file: %w", err)
			}

			content := string(data)
			var newContent string

			switch operation {
			case "add_import":
				newContent = addImport(content, value)
			case "add_function":
				newContent = content + "\n" + value + "\n"
			case "append_content":
				newContent = content + "\n" + value
			default:
				return "", fmt.Errorf("unsupported operation: %s", operation)
			}

			// Write file
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return "", fmt.Errorf("failed to write file: %w", err)
			}

			logger.Debug("smart_edit: performed %s on %s", operation, path)
			return fmt.Sprintf("Successfully performed %s on %s\nFile size: %d → %d bytes", operation, path, len(content), len(newContent)), nil
		},
	})
}

func addImport(content, importPath string) string {
	// Simple implementation for Go imports
	if strings.Contains(content, "import (") {
		// Add to existing import block
		importBlock := "import ("
		replacement := fmt.Sprintf("import (\n\t\"%s\"", importPath)
		return strings.Replace(content, importBlock, replacement, 1)
	} else if strings.Contains(content, "package ") {
		// Add new import block after package declaration
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "package ") {
				lines = append(lines[:i+1], append([]string{"", fmt.Sprintf("import \"%s\"", importPath), ""}, lines[i+1:]...)...)
				break
			}
		}
		return strings.Join(lines, "\n")
	}
	return content
}
