package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vpoluyaktov/vibebot/internal/llm"
)

// RegisterFileTools adds file operation tools to the registry
func RegisterFileTools(registry *Registry, workspaceDir string) {
	// read_file tool
	registry.Register("read_file", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "read_file",
				Description: "Read the contents of a file at the given path.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The file path to read",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, ok := args["path"].(string)
			if !ok {
				return "", fmt.Errorf("path must be a string")
			}

			// Resolve path relative to workspace if not absolute
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspaceDir, path)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("failed to read file: %w", err)
			}

			return string(data), nil
		},
	})

	// write_file tool
	registry.Register("write_file", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "write_file",
				Description: "Write content to a file at the given path. Creates parent directories if needed.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The file path to write to",
						},
						"content": map[string]interface{}{
							"type":        "string",
							"description": "The content to write",
						},
					},
					"required": []string{"path", "content"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, ok := args["path"].(string)
			if !ok {
				return "", fmt.Errorf("path must be a string")
			}
			content, ok := args["content"].(string)
			if !ok {
				return "", fmt.Errorf("content must be a string")
			}

			// Resolve path relative to workspace if not absolute
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspaceDir, path)
			}

			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return "", fmt.Errorf("failed to create directories: %w", err)
			}

			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return "", fmt.Errorf("failed to write file: %w", err)
			}

			return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
		},
	})

	// edit_file tool
	registry.Register("edit_file", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "edit_file",
				Description: "Edit a file by replacing old_text with new_text. The old_text must exist exactly in the file.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The file path to edit",
						},
						"old_text": map[string]interface{}{
							"type":        "string",
							"description": "The exact text to find and replace",
						},
						"new_text": map[string]interface{}{
							"type":        "string",
							"description": "The text to replace with",
						},
					},
					"required": []string{"path", "old_text", "new_text"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, ok := args["path"].(string)
			if !ok {
				return "", fmt.Errorf("path must be a string")
			}
			oldText, ok := args["old_text"].(string)
			if !ok {
				return "", fmt.Errorf("old_text must be a string")
			}
			newText, ok := args["new_text"].(string)
			if !ok {
				return "", fmt.Errorf("new_text must be a string")
			}

			// Resolve path relative to workspace if not absolute
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspaceDir, path)
			}

			// Read file
			data, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("failed to read file: %w", err)
			}

			content := string(data)

			// Check if old_text exists
			if !strings.Contains(content, oldText) {
				return "", fmt.Errorf("old_text not found in file")
			}

			// Replace
			newContent := strings.Replace(content, oldText, newText, 1)

			// Write back
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return "", fmt.Errorf("failed to write file: %w", err)
			}

			return fmt.Sprintf("Successfully edited %s", path), nil
		},
	})

	// list_dir tool
	registry.Register("list_dir", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "list_dir",
				Description: "List the contents of a directory.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "The directory path to list",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, ok := args["path"].(string)
			if !ok {
				return "", fmt.Errorf("path must be a string")
			}

			// Resolve path relative to workspace if not absolute
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspaceDir, path)
			}

			entries, err := os.ReadDir(path)
			if err != nil {
				return "", fmt.Errorf("failed to read directory: %w", err)
			}

			var result strings.Builder
			for _, entry := range entries {
				if entry.IsDir() {
					result.WriteString(entry.Name() + "/\n")
				} else {
					info, _ := entry.Info()
					result.WriteString(fmt.Sprintf("%s (%d bytes)\n", entry.Name(), info.Size()))
				}
			}

			return result.String(), nil
		},
	})
}
