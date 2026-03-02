package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
)

func RegisterMultiFileRead(registry *Registry, workspaceDir string) {
	registry.Register("multi_file_read", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "multi_file_read",
				Description: "Read multiple files in a single request. Returns structured results with file paths and contents.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"paths": map[string]interface{}{
							"type":        "array",
							"description": "Array of file paths to read",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"include_metadata": map[string]interface{}{
							"type":        "boolean",
							"description": "Include file size and modification time (default: false)",
							"default":     false,
						},
					},
					"required": []string{"paths"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			pathsRaw, ok := args["paths"]
			if !ok {
				return "", fmt.Errorf("paths parameter is required")
			}

			pathsArray, ok := pathsRaw.([]interface{})
			if !ok {
				return "", fmt.Errorf("paths must be an array")
			}

			if len(pathsArray) == 0 {
				return "", fmt.Errorf("paths array cannot be empty")
			}

			includeMetadata := false
			if meta, ok := args["include_metadata"].(bool); ok {
				includeMetadata = meta
			}

			var results []string
			successCount := 0

			for i, pathRaw := range pathsArray {
				path, ok := pathRaw.(string)
				if !ok {
					results = append(results, fmt.Sprintf("[%d] ERROR: path must be a string", i+1))
					continue
				}

				// Resolve path relative to workspace if not absolute
				if !filepath.IsAbs(path) {
					path = filepath.Join(workspaceDir, path)
				}

				// Read file
				data, err := os.ReadFile(path)
				if err != nil {
					results = append(results, fmt.Sprintf("[%d] %s: ERROR - %v", i+1, pathRaw, err))
					logger.Debug("multi_file_read: failed to read %s: %v", path, err)
					continue
				}

				// Build result
				var result strings.Builder
				result.WriteString(fmt.Sprintf("[%d] %s:\n", i+1, pathRaw))

				if includeMetadata {
					info, _ := os.Stat(path)
					if info != nil {
						result.WriteString(fmt.Sprintf("Size: %d bytes, Modified: %s\n", info.Size(), info.ModTime().Format(time.RFC3339)))
					}
				}

				result.WriteString(string(data))
				results = append(results, result.String())
				successCount++
			}

			// Format combined results
			var output strings.Builder
			output.WriteString(fmt.Sprintf("Read %d/%d files successfully\n\n", successCount, len(pathsArray)))
			for _, result := range results {
				output.WriteString(result)
				output.WriteString("\n---\n")
			}

			logger.Debug("multi_file_read: read %d/%d files", successCount, len(pathsArray))
			return output.String(), nil
		},
	})
}
