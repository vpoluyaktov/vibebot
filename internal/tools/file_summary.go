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

func RegisterFileSummary(registry *Registry, workspaceDir string) {
	registry.Register("file_summary", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "file_summary",
				Description: "Get file metadata without reading full contents. Returns imports, exported symbols, function signatures, type definitions.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "File path to summarize",
						},
						"include_private": map[string]interface{}{
							"type":        "boolean",
							"description": "Include private (unexported) symbols (default: false)",
							"default":     false,
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

			includePrivate := false
			if priv, ok := args["include_private"].(bool); ok {
				includePrivate = priv
			}

			// Resolve path
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspaceDir, path)
			}

			// Check if file exists
			if _, err := os.Stat(path); err != nil {
				return "", fmt.Errorf("file not found: %w", err)
			}

			// Try to parse with tree-sitter
			tree, lang, err := parser.ParseFile(path)
			if err != nil {
				// Fall back to generic summary for unsupported languages
				return summarizeGenericFile(path)
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("failed to read file: %w", err)
			}

			// Extract symbols and imports
			symbols := parser.ExtractSymbols(tree, lang, content, includePrivate)
			imports := parser.GetImports(tree, lang, content)

			// Build summary
			var output strings.Builder
			output.WriteString(fmt.Sprintf("File: %s\n", filepath.Base(path)))
			output.WriteString(fmt.Sprintf("Language: %s\n\n", lang.Name))

			// Imports
			if len(imports) > 0 {
				output.WriteString("Imports:\n")
				for _, imp := range imports {
					output.WriteString(fmt.Sprintf("  %s\n", imp))
				}
				output.WriteString("\n")
			}

			// Group symbols by type
			types := []parser.Symbol{}
			functions := []parser.Symbol{}
			methods := []parser.Symbol{}
			constants := []parser.Symbol{}
			variables := []parser.Symbol{}
			other := []parser.Symbol{}

			for _, sym := range symbols {
				switch sym.Type {
				case "struct", "class", "interface", "trait":
					types = append(types, sym)
				case "function":
					functions = append(functions, sym)
				case "method":
					methods = append(methods, sym)
				case "const":
					constants = append(constants, sym)
				case "var", "let", "field":
					variables = append(variables, sym)
				default:
					other = append(other, sym)
				}
			}

			// Output sections
			if len(types) > 0 {
				output.WriteString("Types:\n")
				for _, t := range types {
					output.WriteString(fmt.Sprintf("  %s (line %d)\n", t.Signature, t.StartLine))
				}
				output.WriteString("\n")
			}

			if len(constants) > 0 {
				output.WriteString("Constants:\n")
				for _, c := range constants {
					output.WriteString(fmt.Sprintf("  %s (line %d)\n", c.Signature, c.StartLine))
				}
				output.WriteString("\n")
			}

			if len(variables) > 0 {
				output.WriteString("Variables:\n")
				for _, v := range variables {
					output.WriteString(fmt.Sprintf("  %s (line %d)\n", v.Signature, v.StartLine))
				}
				output.WriteString("\n")
			}

			if len(functions) > 0 {
				output.WriteString("Functions:\n")
				for _, f := range functions {
					output.WriteString(fmt.Sprintf("  %s (line %d)\n", f.Signature, f.StartLine))
				}
				output.WriteString("\n")
			}

			if len(methods) > 0 {
				output.WriteString("Methods:\n")
				for _, m := range methods {
					output.WriteString(fmt.Sprintf("  %s (line %d)\n", m.Signature, m.StartLine))
				}
				output.WriteString("\n")
			}

			if len(other) > 0 {
				output.WriteString("Other:\n")
				for _, o := range other {
					output.WriteString(fmt.Sprintf("  %s %s (line %d)\n", o.Type, o.Name, o.StartLine))
				}
				output.WriteString("\n")
			}

			summary := output.String()
			logger.Debug("file_summary: summarized %s (%s) - %d symbols", path, lang.Name, len(symbols))
			return summary, nil
		},
	})
}

func summarizeGenericFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	lineCount := len(lines)

	// Adjust for files ending with newline
	if len(content) > 0 && strings.HasSuffix(string(content), "\n") && lineCount > 0 && lines[lineCount-1] == "" {
		lineCount--
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("File: %s\n", filepath.Base(path)))
	output.WriteString(fmt.Sprintf("Type: Generic/Unsupported\n"))
	output.WriteString(fmt.Sprintf("Size: %d bytes\n", info.Size()))
	output.WriteString(fmt.Sprintf("Lines: %d\n", lineCount))

	return output.String(), nil
}
