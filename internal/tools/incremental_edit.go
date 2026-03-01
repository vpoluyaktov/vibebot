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

func RegisterIncrementalEdit(registry *Registry, workspaceDir string) {
	registry.Register("incremental_edit", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "incremental_edit",
				Description: "Edit specific line ranges in a file without search/replace. More precise and token-efficient than edit_file. Specify exact line numbers to replace. Supports inserting, replacing, or deleting lines.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "File path to edit",
						},
						"start_line": map[string]interface{}{
							"type":        "integer",
							"description": "Starting line number (1-indexed)",
						},
						"end_line": map[string]interface{}{
							"type":        "integer",
							"description": "Ending line number (1-indexed, inclusive). Use same as start_line to insert.",
						},
						"new_content": map[string]interface{}{
							"type":        "string",
							"description": "New content to insert. Empty string deletes the lines.",
						},
					},
					"required": []string{"path", "start_line", "end_line", "new_content"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, ok := args["path"].(string)
			if !ok {
				return "", fmt.Errorf("path must be a string")
			}

			startLine, ok := args["start_line"].(float64)
			if !ok {
				return "", fmt.Errorf("start_line must be an integer")
			}

			endLine, ok := args["end_line"].(float64)
			if !ok {
				return "", fmt.Errorf("end_line must be an integer")
			}

			newContent, ok := args["new_content"].(string)
			if !ok {
				return "", fmt.Errorf("new_content must be a string")
			}

			// Resolve path
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspaceDir, path)
			}

			// Validate line numbers
			start := int(startLine)
			end := int(endLine)
			if start < 1 {
				return "", fmt.Errorf("start_line must be >= 1")
			}
			if end < 1 {
				return "", fmt.Errorf("end_line must be >= 1")
			}

			// If end < start, this is an insert operation at start position
			isInsert := end < start
			if isInsert {
				end = start - 1
			}

			// Read file
			content, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("failed to read file: %w", err)
			}

			lines := strings.Split(string(content), "\n")
			totalLines := len(lines)

			// Adjust for files that don't end with newline
			if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
				// File doesn't end with newline, line count is accurate
			} else if totalLines > 0 && lines[totalLines-1] == "" {
				// File ends with newline, split creates empty last element
				totalLines--
				lines = lines[:totalLines]
			}

			// Validate line range
			if start > totalLines+1 {
				return "", fmt.Errorf("start_line %d exceeds file length %d", start, totalLines)
			}
			if end > totalLines {
				end = totalLines
			}

			// Prepare new content lines
			var newLines []string
			if newContent != "" {
				newLines = strings.Split(newContent, "\n")
				// Remove trailing empty line if newContent ends with \n
				if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
					newLines = newLines[:len(newLines)-1]
				}
			}

			// Build result
			var result []string

			// Lines before the edit
			if start > 1 {
				result = append(result, lines[:start-1]...)
			}

			// Insert new content
			if len(newLines) > 0 {
				result = append(result, newLines...)
			}

			// Lines after the edit
			if end < totalLines {
				result = append(result, lines[end:]...)
			}

			// Write back
			newFileContent := strings.Join(result, "\n")
			if len(content) > 0 && strings.HasSuffix(string(content), "\n") {
				newFileContent += "\n"
			}

			if err := os.WriteFile(path, []byte(newFileContent), 0644); err != nil {
				return "", fmt.Errorf("failed to write file: %w", err)
			}

			// Calculate changes
			linesRemoved := end - start + 1
			linesAdded := len(newLines)
			netChange := linesAdded - linesRemoved

			var operation string
			if linesAdded == 0 {
				operation = "Deleted"
			} else if linesRemoved == 0 || start > totalLines {
				operation = "Inserted"
			} else {
				operation = "Replaced"
			}

			logger.Debug("incremental_edit: %s lines %d-%d in %s", operation, start, end, path)

			return fmt.Sprintf("%s lines %d-%d in %s\nLines changed: %+d (removed: %d, added: %d)\nNew file length: %d lines",
				operation, start, end, filepath.Base(path), netChange, linesRemoved, linesAdded, len(result)), nil
		},
	})
}
