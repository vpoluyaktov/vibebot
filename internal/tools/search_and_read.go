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

func RegisterSearchAndRead(registry *Registry, workspaceDir string) {
	registry.Register("search_and_read", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "search_and_read",
				Description: "Search for files matching a pattern and read them in one operation. Combines list_dir + grep + read_file into a single call.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"pattern": map[string]interface{}{
							"type":        "string",
							"description": "File pattern to match (e.g., '*.go', 'test_*.py')",
						},
						"directory": map[string]interface{}{
							"type":        "string",
							"description": "Directory to search in (default: workspace root)",
							"default":     ".",
						},
						"max_files": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of files to read (default: 10)",
							"default":     10,
						},
						"grep_filter": map[string]interface{}{
							"type":        "string",
							"description": "Optional: only read files containing this text",
						},
					},
					"required": []string{"pattern"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			pattern, ok := args["pattern"].(string)
			if !ok {
				return "", fmt.Errorf("pattern must be a string")
			}

			directory := "."
			if dir, ok := args["directory"].(string); ok {
				directory = dir
			}

			maxFiles := 10
			if max, ok := args["max_files"].(float64); ok {
				maxFiles = int(max)
			}

			grepFilter := ""
			if filter, ok := args["grep_filter"].(string); ok {
				grepFilter = filter
			}

			// Resolve directory
			if !filepath.IsAbs(directory) {
				directory = filepath.Join(workspaceDir, directory)
			}

			// Find matching files
			var matchedFiles []string
			err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil // Skip errors
				}
				if info.IsDir() {
					return nil
				}

				matched, _ := filepath.Match(pattern, filepath.Base(path))
				if matched {
					matchedFiles = append(matchedFiles, path)
				}

				return nil
			})

			if err != nil {
				return "", fmt.Errorf("failed to search directory: %w", err)
			}

			if len(matchedFiles) == 0 {
				return fmt.Sprintf("No files matching pattern '%s' found in %s", pattern, directory), nil
			}

			// Apply grep filter if specified
			var filesToRead []string
			if grepFilter != "" {
				for _, path := range matchedFiles {
					data, err := os.ReadFile(path)
					if err != nil {
						continue
					}
					if strings.Contains(string(data), grepFilter) {
						filesToRead = append(filesToRead, path)
					}
				}
			} else {
				filesToRead = matchedFiles
			}

			// Limit number of files
			if len(filesToRead) > maxFiles {
				filesToRead = filesToRead[:maxFiles]
			}

			if len(filesToRead) == 0 {
				return fmt.Sprintf("Found %d files matching '%s', but none contain '%s'", len(matchedFiles), pattern, grepFilter), nil
			}

			// Read files
			var results []string
			for i, path := range filesToRead {
				data, err := os.ReadFile(path)
				if err != nil {
					results = append(results, fmt.Sprintf("[%d] %s: ERROR - %v", i+1, path, err))
					continue
				}

				relPath, _ := filepath.Rel(workspaceDir, path)
				results = append(results, fmt.Sprintf("[%d] %s:\n%s", i+1, relPath, string(data)))
			}

			var output strings.Builder
			output.WriteString(fmt.Sprintf("Found %d files matching '%s'", len(matchedFiles), pattern))
			if grepFilter != "" {
				output.WriteString(fmt.Sprintf(" (filtered to %d containing '%s')", len(filesToRead), grepFilter))
			}
			output.WriteString(fmt.Sprintf(", reading %d files:\n\n", len(filesToRead)))

			for _, result := range results {
				output.WriteString(result)
				output.WriteString("\n---\n")
			}

			logger.Debug("search_and_read: found %d files, read %d", len(matchedFiles), len(filesToRead))
			return output.String(), nil
		},
	})
}
