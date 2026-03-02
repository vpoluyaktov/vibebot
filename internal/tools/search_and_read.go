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

func RegisterSearchAndRead(registry *Registry, workspaceDir string) {
	registry.Register("search_and_read", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "search_and_read",
				Description: "Search for files matching a pattern and read them. Returns full content, summaries, or outlines based on mode.",
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
						"mode": map[string]interface{}{
							"type":        "string",
							"description": "Output mode: full (entire file), summary (file metadata), outline (hierarchical structure)",
							"enum":        []string{"full", "summary", "outline"},
							"default":     "full",
						},
						"include_private": map[string]interface{}{
							"type":        "boolean",
							"description": "For summary/outline modes: include private symbols (default: false)",
							"default":     false,
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

			mode := "full"
			if m, ok := args["mode"].(string); ok {
				mode = m
			}

			includePrivate := false
			if priv, ok := args["include_private"].(bool); ok {
				includePrivate = priv
			}

			// Resolve directory
			if !filepath.IsAbs(directory) {
				directory = filepath.Join(workspaceDir, directory)
			}

			// Find matching files
			var matchedFiles []string
			err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					if info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "vendor" {
						return filepath.SkipDir
					}
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

			// Read files based on mode
			var results []string
			tokensSaved := 0

			for i, path := range filesToRead {
				relPath, _ := filepath.Rel(workspaceDir, path)

				switch mode {
				case "summary":
					summary, tokens := getFileSummary(path, relPath, includePrivate)
					results = append(results, fmt.Sprintf("[%d] %s", i+1, summary))
					tokensSaved += tokens

				case "outline":
					outline, tokens := getFileOutline(path, relPath, includePrivate)
					results = append(results, fmt.Sprintf("[%d] %s", i+1, outline))
					tokensSaved += tokens

				default: // full
					data, err := os.ReadFile(path)
					if err != nil {
						results = append(results, fmt.Sprintf("[%d] %s: ERROR - %v", i+1, relPath, err))
						continue
					}
					results = append(results, fmt.Sprintf("[%d] %s:\n%s", i+1, relPath, string(data)))
				}
			}

			var output strings.Builder
			output.WriteString(fmt.Sprintf("Found %d files matching '%s'", len(matchedFiles), pattern))
			if grepFilter != "" {
				output.WriteString(fmt.Sprintf(" (filtered to %d containing '%s')", len(filesToRead), grepFilter))
			}
			output.WriteString(fmt.Sprintf(", reading %d files in '%s' mode:\n\n", len(filesToRead), mode))

			if mode != "full" && tokensSaved > 0 {
				output.WriteString(fmt.Sprintf("💡 Token savings: ~%d tokens vs full mode\n\n", tokensSaved))
			}

			for _, result := range results {
				output.WriteString(result)
				output.WriteString("\n---\n")
			}

			logger.Debug("search_and_read: found %d files, read %d in %s mode", len(matchedFiles), len(filesToRead), mode)
			return output.String(), nil
		},
	})
}

func getFileSummary(path, relPath string, includePrivate bool) (string, int) {
	tree, lang, err := parser.ParseFile(path)
	if err != nil {
		// Not a supported language, return basic info
		info, _ := os.Stat(path)
		return fmt.Sprintf("%s (unsupported format, %d bytes)", relPath, info.Size()), 0
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("%s: ERROR - %v", relPath, err), 0
	}

	symbols := parser.ExtractSymbols(tree, lang, content, includePrivate)
	imports := parser.GetImports(tree, lang, content)

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("%s (%s)\n", relPath, lang.Name))

	if len(imports) > 0 {
		summary.WriteString(fmt.Sprintf("  Imports: %d\n", len(imports)))
	}

	// Count by type
	typeCounts := make(map[string]int)
	for _, sym := range symbols {
		typeCounts[sym.Type]++
	}

	for symType, count := range typeCounts {
		summary.WriteString(fmt.Sprintf("  %ss: %d\n", symType, count))
	}

	// Estimate tokens saved (rough estimate: full file vs summary)
	fullFileTokens := len(content) / 4 // rough estimate
	summaryTokens := len(summary.String()) / 4
	tokensSaved := fullFileTokens - summaryTokens
	if tokensSaved < 0 {
		tokensSaved = 0
	}

	return summary.String(), tokensSaved
}

func getFileOutline(path, relPath string, includePrivate bool) (string, int) {
	tree, lang, err := parser.ParseFile(path)
	if err != nil {
		info, _ := os.Stat(path)
		return fmt.Sprintf("%s (unsupported format, %d bytes)", relPath, info.Size()), 0
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("%s: ERROR - %v", relPath, err), 0
	}

	symbols := parser.ExtractSymbols(tree, lang, content, includePrivate)

	var outline strings.Builder
	outline.WriteString(fmt.Sprintf("%s (%s)\n", relPath, lang.Name))

	// Group by type
	types := []parser.Symbol{}
	functions := []parser.Symbol{}
	methods := []parser.Symbol{}

	for _, sym := range symbols {
		switch sym.Type {
		case "class", "struct", "interface", "trait":
			types = append(types, sym)
		case "function":
			functions = append(functions, sym)
		case "method":
			methods = append(methods, sym)
		}
	}

	if len(types) > 0 {
		outline.WriteString("  Types:\n")
		for _, t := range types {
			outline.WriteString(fmt.Sprintf("    - %s (line %d)\n", t.Name, t.StartLine))
		}
	}

	if len(functions) > 0 {
		outline.WriteString("  Functions:\n")
		for _, f := range functions {
			outline.WriteString(fmt.Sprintf("    - %s (line %d)\n", f.Name, f.StartLine))
		}
	}

	if len(methods) > 0 {
		outline.WriteString("  Methods:\n")
		for _, m := range methods {
			outline.WriteString(fmt.Sprintf("    - %s (line %d)\n", m.Name, m.StartLine))
		}
	}

	fullFileTokens := len(content) / 4
	outlineTokens := len(outline.String()) / 4
	tokensSaved := fullFileTokens - outlineTokens
	if tokensSaved < 0 {
		tokensSaved = 0
	}

	return outline.String(), tokensSaved
}
