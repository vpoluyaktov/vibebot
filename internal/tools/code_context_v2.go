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

func RegisterCodeContextV2(registry *Registry, workspaceDir string) {
	registry.Register("code_context", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "code_context",
				Description: "Get relevant code context for a symbol/function using AST parsing. Returns definition, usages with line numbers and context. Supports Go, Python, JavaScript, TypeScript, Java, C, C++, Rust. Much more efficient than reading full files.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"symbol": map[string]interface{}{
							"type":        "string",
							"description": "Symbol/function name to find context for",
						},
						"include_tests": map[string]interface{}{
							"type":        "boolean",
							"description": "Include test files (default: true)",
							"default":     true,
						},
						"context_lines": map[string]interface{}{
							"type":        "integer",
							"description": "Number of context lines around usages (default: 3)",
							"default":     3,
						},
						"max_usages": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of usage examples to show (default: 10)",
							"default":     10,
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

			includeTests := true
			if tests, ok := args["include_tests"].(bool); ok {
				includeTests = tests
			}

			contextLines := 3
			if lines, ok := args["context_lines"].(float64); ok {
				contextLines = int(lines)
			}

			maxUsages := 10
			if max, ok := args["max_usages"].(float64); ok {
				maxUsages = int(max)
			}

			// Find definitions and usages
			var definitions []SymbolLocation
			var usages []SymbolLocation

			err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					if info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "vendor" {
						return filepath.SkipDir
					}
					return nil
				}

				// Skip test files if not included
				if !includeTests && isTestFile(path) {
					return nil
				}

				// Try to parse with tree-sitter
				tree, lang, err := parser.ParseFile(path)
				if err != nil {
					// Not a supported language, skip
					return nil
				}

				content, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				// Extract symbols and check for definition
				symbols := parser.ExtractSymbols(tree, lang, content, true)
				for _, sym := range symbols {
					if sym.Name == symbol {
						relPath, _ := filepath.Rel(workspaceDir, path)
						definitions = append(definitions, SymbolLocation{
							FilePath:  relPath,
							Symbol:    sym,
							Content:   content,
							IsDefn:    true,
							Language:  lang.Name,
						})
					}
				}

				// Find usages (simple text search in parsed files)
				lines := strings.Split(string(content), "\n")
				for i, line := range lines {
					if strings.Contains(line, symbol) {
						// Check if this is not a definition line
						isDef := false
						for _, def := range definitions {
							if def.FilePath == filepath.Base(path) && int(def.Symbol.StartLine) == i+1 {
								isDef = true
								break
							}
						}

						if !isDef {
							relPath, _ := filepath.Rel(workspaceDir, path)
							usages = append(usages, SymbolLocation{
								FilePath:     relPath,
								LineNum:      i + 1,
								Line:         line,
								Content:      content,
								ContextLines: extractContextLines(lines, i, contextLines),
								Language:     lang.Name,
							})
						}
					}
				}

				return nil
			})

			if err != nil {
				return "", fmt.Errorf("failed to search workspace: %w", err)
			}

			// Format output
			var output strings.Builder
			
			if len(definitions) == 0 && len(usages) == 0 {
				return fmt.Sprintf("Symbol '%s' not found in workspace", symbol), nil
			}

			output.WriteString(fmt.Sprintf("Code Context for '%s':\n\n", symbol))

			// Show definitions
			if len(definitions) > 0 {
				output.WriteString(fmt.Sprintf("=== DEFINITIONS (%d) ===\n\n", len(definitions)))
				for i, def := range definitions {
					output.WriteString(fmt.Sprintf("[D%d] %s:%d-%d (%s)\n", i+1, def.FilePath, def.Symbol.StartLine, def.Symbol.EndLine, def.Language))
					output.WriteString(fmt.Sprintf("Type: %s\n", def.Symbol.Type))
					output.WriteString(fmt.Sprintf("Signature: %s\n", def.Symbol.Signature))
					output.WriteString("\nDefinition:\n")
					
					defCode := extractDefinition(def.Content, def.Symbol.StartLine, def.Symbol.EndLine)
					output.WriteString(defCode)
					output.WriteString("\n---\n\n")
				}
			}

			// Show usages
			if len(usages) > 0 {
				displayUsages := len(usages)
				if displayUsages > maxUsages {
					displayUsages = maxUsages
				}

				output.WriteString(fmt.Sprintf("=== USAGES (%d total, showing %d) ===\n\n", len(usages), displayUsages))
				for i := 0; i < displayUsages; i++ {
					usage := usages[i]
					output.WriteString(fmt.Sprintf("[U%d] %s:%d (%s)\n", i+1, usage.FilePath, usage.LineNum, usage.Language))
					for _, ctxLine := range usage.ContextLines {
						output.WriteString(ctxLine + "\n")
					}
					output.WriteString("\n")
				}

				if len(usages) > maxUsages {
					output.WriteString(fmt.Sprintf("... and %d more usages (use max_usages to see more)\n", len(usages)-maxUsages))
				}
			}

			logger.Debug("code_context: found %d definitions and %d usages for '%s'", len(definitions), len(usages), symbol)
			return output.String(), nil
		},
	})
}

type SymbolLocation struct {
	FilePath     string
	Symbol       parser.Symbol
	LineNum      int
	Line         string
	Content      []byte
	ContextLines []string
	IsDefn       bool
	Language     string
}

func isTestFile(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(base, "_test.") || strings.Contains(base, ".test.") || 
	       strings.Contains(base, "_spec.") || strings.Contains(base, ".spec.")
}

func extractContextLines(lines []string, lineIdx, contextSize int) []string {
	var result []string
	
	start := lineIdx - contextSize
	if start < 0 {
		start = 0
	}
	end := lineIdx + contextSize + 1
	if end > len(lines) {
		end = len(lines)
	}

	for i := start; i < end; i++ {
		if i == lineIdx {
			result = append(result, fmt.Sprintf("> %4d | %s", i+1, lines[i]))
		} else {
			result = append(result, fmt.Sprintf("  %4d | %s", i+1, lines[i]))
		}
	}

	return result
}
