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

func RegisterCodeContext(registry *Registry, workspaceDir string) {
	registry.Register("code_context", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "code_context",
				Description: "Get relevant code context for a symbol/function. Finds definition, usages, and related code automatically.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol": map[string]interface{}{
							"type":        "string",
							"description": "Symbol/function name to find context for",
						},
						"language": map[string]interface{}{
							"type":        "string",
							"description": "Programming language (go, python, javascript, etc.)",
							"default":     "go",
						},
						"include_tests": map[string]interface{}{
							"type":        "boolean",
							"description": "Include test files (default: true)",
							"default":     true,
						},
					},
					"required": []string{"symbol"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			symbol, ok := args["symbol"].(string)
			if !ok {
				return "", fmt.Errorf("symbol must be a string")
			}

			language := "go"
			if lang, ok := args["language"].(string); ok {
				language = lang
			}

			includeTests := true
			if tests, ok := args["include_tests"].(bool); ok {
				includeTests = tests
			}

			// Determine file extension based on language
			ext := getExtensionForLanguage(language)

			// Search for symbol in files
			var foundFiles []string
			err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					return nil
				}

				// Check file extension
				if !strings.HasSuffix(path, ext) {
					return nil
				}

				// Skip test files if not included
				if !includeTests && strings.Contains(path, "_test"+ext) {
					return nil
				}

				// Read and search for symbol
				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				if strings.Contains(string(data), symbol) {
					foundFiles = append(foundFiles, path)
				}

				return nil
			})

			if err != nil {
				return "", fmt.Errorf("failed to search for symbol: %w", err)
			}

			if len(foundFiles) == 0 {
				return fmt.Sprintf("Symbol '%s' not found in %s files", symbol, language), nil
			}

			// Read found files
			var results []string
			for i, path := range foundFiles {
				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}

				relPath, _ := filepath.Rel(workspaceDir, path)
				results = append(results, fmt.Sprintf("[%d] %s:\n%s", i+1, relPath, string(data)))

				// Limit to 5 files to avoid overwhelming output
				if i >= 4 {
					break
				}
			}

			var output strings.Builder
			output.WriteString(fmt.Sprintf("Found symbol '%s' in %d %s files:\n\n", symbol, len(foundFiles), language))
			for _, result := range results {
				output.WriteString(result)
				output.WriteString("\n---\n")
			}

			if len(foundFiles) > 5 {
				output.WriteString(fmt.Sprintf("\n(Showing first 5 of %d files)", len(foundFiles)))
			}

			logger.Debug("code_context: found symbol '%s' in %d files", symbol, len(foundFiles))
			return output.String(), nil
		},
	})
}

func getExtensionForLanguage(language string) string {
	extensions := map[string]string{
		"go":         ".go",
		"python":     ".py",
		"javascript": ".js",
		"typescript": ".ts",
		"java":       ".java",
		"c":          ".c",
		"cpp":        ".cpp",
		"rust":       ".rs",
		"ruby":       ".rb",
	}

	if ext, ok := extensions[strings.ToLower(language)]; ok {
		return ext
	}
	return ".go" // default
}
