package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
	"github.com/vpoluyaktov/vibebot/internal/tools/parser"
)

func RegisterFileOutline(registry *Registry, workspaceDir string) {
	registry.Register("file_outline", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "file_outline",
				Description: "Get hierarchical structure of a file showing classes, methods, functions with line numbers.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "File path to outline",
						},
						"include_private": map[string]interface{}{
							"type":        "boolean",
							"description": "Include private/unexported symbols (default: false)",
							"default":     false,
						},
						"max_depth": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum nesting depth to show (default: unlimited)",
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

			maxDepth := -1 // unlimited
			if depth, ok := args["max_depth"].(float64); ok {
				maxDepth = int(depth)
			}

			// Resolve path
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspaceDir, path)
			}

			// Check if file exists
			if _, err := os.Stat(path); err != nil {
				return "", fmt.Errorf("file not found: %w", err)
			}

			// Parse file
			tree, lang, err := parser.ParseFile(path)
			if err != nil {
				return "", fmt.Errorf("unsupported file type or parse error: %w", err)
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("failed to read file: %w", err)
			}

			// Extract symbols
			symbols := parser.ExtractSymbols(tree, lang, content, includePrivate)

			// Build hierarchical outline
			outline := buildOutline(symbols, maxDepth)

			var output strings.Builder
			output.WriteString(fmt.Sprintf("File Outline: %s\n", filepath.Base(path)))
			output.WriteString(fmt.Sprintf("Language: %s\n", lang.Name))
			output.WriteString(fmt.Sprintf("Total symbols: %d\n\n", len(symbols)))
			output.WriteString(outline)

			logger.Debug("file_outline: generated outline for %s with %d symbols", path, len(symbols))
			return output.String(), nil
		},
	})
}

type OutlineNode struct {
	Symbol   parser.Symbol
	Children []OutlineNode
}

func buildOutline(symbols []parser.Symbol, maxDepth int) string {
	// Group symbols by type and hierarchy
	var topLevel []parser.Symbol
	methodsByReceiver := make(map[string][]parser.Symbol)

	for _, sym := range symbols {
		if sym.Type == "method" && sym.Receiver != "" {
			// Extract receiver type name
			receiverType := extractReceiverType(sym.Receiver)
			methodsByReceiver[receiverType] = append(methodsByReceiver[receiverType], sym)
		} else {
			topLevel = append(topLevel, sym)
		}
	}

	// Sort top-level symbols by line number
	sort.Slice(topLevel, func(i, j int) bool {
		return topLevel[i].StartLine < topLevel[j].StartLine
	})

	// Build outline
	var output strings.Builder
	for _, sym := range topLevel {
		renderSymbol(&output, sym, 0, maxDepth, methodsByReceiver)
	}

	return output.String()
}

func renderSymbol(output *strings.Builder, sym parser.Symbol, depth int, maxDepth int, methodsByReceiver map[string][]parser.Symbol) {
	if maxDepth >= 0 && depth > maxDepth {
		return
	}

	indent := strings.Repeat("  ", depth)
	icon := getSymbolIcon(sym.Type)

	// Format the symbol
	var exported string
	if sym.IsExported {
		exported = "+"
	} else {
		exported = "-"
	}

	output.WriteString(fmt.Sprintf("%s%s %s %s (line %d)\n", indent, icon, exported, sym.Name, sym.StartLine))

	// If this is a class/struct/interface, show its methods
	if sym.Type == "class" || sym.Type == "struct" || sym.Type == "interface" || sym.Type == "trait" {
		if methods, exists := methodsByReceiver[sym.Name]; exists {
			// Sort methods by line number
			sort.Slice(methods, func(i, j int) bool {
				return methods[i].StartLine < methods[j].StartLine
			})

			for _, method := range methods {
				renderSymbol(output, method, depth+1, maxDepth, methodsByReceiver)
			}
		}
	}
}

func getSymbolIcon(symbolType string) string {
	icons := map[string]string{
		"class":     "📦",
		"struct":    "📦",
		"interface": "🔌",
		"trait":     "🔌",
		"function":  "ƒ",
		"method":    "→",
		"const":     "◆",
		"var":       "◇",
		"let":       "◇",
		"field":     "·",
	}

	if icon, ok := icons[symbolType]; ok {
		return icon
	}
	return "•"
}

func extractReceiverType(receiver string) string {
	// Extract type name from receiver
	// Examples: "(u *User)", "(*User)", "(User)"
	receiver = strings.TrimSpace(receiver)
	receiver = strings.Trim(receiver, "()")

	// Remove pointer indicator
	receiver = strings.TrimPrefix(receiver, "*")

	// Split by space and take last part
	parts := strings.Fields(receiver)
	if len(parts) > 0 {
		typeName := parts[len(parts)-1]
		typeName = strings.TrimPrefix(typeName, "*")
		return typeName
	}

	return receiver
}
