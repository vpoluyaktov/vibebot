package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
	"github.com/vpoluyaktov/vibebot/internal/tools/parser"
)

func RegisterSymbolDefinition(registry *Registry, workspaceDir string) {
	registry.Register("symbol_definition", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "symbol_definition",
				Description: "Find exact definition of a symbol (function, class, method, type). Returns only the definition block with line numbers.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol": map[string]interface{}{
							"type":        "string",
							"description": "Symbol name to find (e.g., 'NewUser', 'User', 'createUser')",
						},
						"file_pattern": map[string]interface{}{
							"type":        "string",
							"description": "Optional file pattern to narrow search (e.g., '*.go', '*.py')",
						},
						"include_body": map[string]interface{}{
							"type":        "boolean",
							"description": "Include the full function/method body (default: true)",
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

			filePattern := ""
			if pattern, ok := args["file_pattern"].(string); ok {
				filePattern = pattern
			}

			includeBody := true
			if body, ok := args["include_body"].(bool); ok {
				includeBody = body
			}

			// Search for files containing the symbol
			var results []SymbolResult
			err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					// Skip common directories
					if info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "vendor" {
						return filepath.SkipDir
					}
					return nil
				}

				// Check file pattern if specified
				if filePattern != "" {
					matched, _ := filepath.Match(filePattern, filepath.Base(path))
					if !matched {
						return nil
					}
				}

				// Try to parse the file
				tree, lang, err := parser.ParseFile(path)
				if err != nil {
					return nil // Skip unsupported files
				}

				content, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				// Extract symbols and find matches
				symbols := parser.ExtractSymbols(tree, lang, content, true) // Include private
				for _, sym := range symbols {
					if sym.Name == symbol {
						relPath, _ := filepath.Rel(workspaceDir, path)
						result := SymbolResult{
							Symbol:   sym,
							FilePath: relPath,
							Content:  content,
						}
						results = append(results, result)
					}
				}

				return nil
			})

			if err != nil {
				return "", fmt.Errorf("failed to search workspace: %w", err)
			}

			if len(results) == 0 {
				return fmt.Sprintf("Symbol '%s' not found in workspace", symbol), nil
			}

			// Format results
			var output strings.Builder
			output.WriteString(fmt.Sprintf("Found %d definition(s) for '%s':\n\n", len(results), symbol))

			for i, result := range results {
				output.WriteString(fmt.Sprintf("[%d] %s:%d-%d\n", i+1, result.FilePath, result.Symbol.StartLine, result.Symbol.EndLine))
				output.WriteString(fmt.Sprintf("Type: %s\n", result.Symbol.Type))
				output.WriteString(fmt.Sprintf("Signature: %s\n", result.Symbol.Signature))

				if includeBody {
					output.WriteString("\nDefinition:\n")
					definition := extractDefinition(result.Content, result.Symbol.StartLine, result.Symbol.EndLine)
					output.WriteString(definition)
				}

				output.WriteString("\n---\n")
			}

			logger.Debug("symbol_definition: found %d definitions for '%s'", len(results), symbol)
			return output.String(), nil
		},
	})
}

type SymbolResult struct {
	Symbol   parser.Symbol
	FilePath string
	Content  []byte
}

func extractDefinition(content []byte, startLine, endLine uint32) string {
	lines := strings.Split(string(content), "\n")

	// Adjust for 1-indexed line numbers
	start := int(startLine) - 1
	end := int(endLine)

	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}

	var result strings.Builder
	for i := start; i < end; i++ {
		result.WriteString(fmt.Sprintf("%4d | %s\n", i+1, lines[i]))
	}

	return result.String()
}
