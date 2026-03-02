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

func RegisterSmartEdit(registry *Registry, workspaceDir string) {
	registry.Register("smart_edit", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "smart_edit",
				Description: "Context-aware editing for adding imports, functions, etc. Automatically detects language and handles syntax.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "File path to edit",
						},
						"operation": map[string]interface{}{
							"type":        "string",
							"description": "Operation type: add_import, add_function, append_content",
							"enum":        []string{"add_import", "add_function", "append_content"},
						},
						"value": map[string]interface{}{
							"type":        "string",
							"description": "Value to add (import path, function code, content to append)",
						},
					},
					"required": []string{"path", "operation", "value"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			operation, _ := args["operation"].(string)
			value, _ := args["value"].(string)

			if path == "" || operation == "" || value == "" {
				return "", fmt.Errorf("path, operation, and value are required")
			}

			// Resolve path
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspaceDir, path)
			}

			// Read file
			data, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("failed to read file: %w", err)
			}

			content := string(data)
			var newContent string

			// Detect language
			lang, _ := parser.GetLanguageByExtension(filepath.Ext(path))

			switch operation {
			case "add_import":
				if lang != nil {
					newContent = addImportMultiLang(content, value, lang.Name)
				} else {
					// Fallback to old behavior for unsupported languages
					newContent = addImportGo(content, value)
				}
			case "add_function":
				newContent = content + "\n" + value + "\n"
			case "append_content":
				newContent = content + "\n" + value
			default:
				return "", fmt.Errorf("unsupported operation: %s", operation)
			}

			// Write file
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return "", fmt.Errorf("failed to write file: %w", err)
			}

			langName := "unknown"
			if lang != nil {
				langName = lang.Name
			}

			logger.Debug("smart_edit: performed %s on %s (%s)", operation, path, langName)
			return fmt.Sprintf("Successfully performed %s on %s (%s)\nFile size: %d → %d bytes",
				operation, filepath.Base(path), langName, len(content), len(newContent)), nil
		},
	})
}

func addImportMultiLang(content, importValue, language string) string {
	switch language {
	case "go":
		return addImportGo(content, importValue)
	case "python":
		return addImportPython(content, importValue)
	case "javascript", "typescript":
		return addImportJavaScript(content, importValue)
	case "java":
		return addImportJava(content, importValue)
	case "rust":
		return addImportRust(content, importValue)
	case "c", "cpp":
		return addImportC(content, importValue)
	default:
		return content
	}
}

func addImportGo(content, importPath string) string {
	// Check if import already exists
	if strings.Contains(content, fmt.Sprintf("\"%s\"", importPath)) {
		return content
	}

	if strings.Contains(content, "import (") {
		// Add to existing import block
		importBlock := "import ("
		replacement := fmt.Sprintf("import (\n\t\"%s\"", importPath)
		return strings.Replace(content, importBlock, replacement, 1)
	} else if strings.Contains(content, "package ") {
		// Add new import block after package declaration
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "package ") {
				lines = append(lines[:i+1], append([]string{"", fmt.Sprintf("import \"%s\"", importPath), ""}, lines[i+1:]...)...)
				break
			}
		}
		return strings.Join(lines, "\n")
	}
	return content
}

func addImportPython(content, importValue string) string {
	// Check if import already exists
	if strings.Contains(content, importValue) {
		return content
	}

	lines := strings.Split(content, "\n")

	// Find the position after existing imports or at the top
	insertPos := 0
	lastImportPos := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") {
			lastImportPos = i
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "#") && lastImportPos >= 0 {
			// Found first non-import, non-comment line after imports
			insertPos = lastImportPos + 1
			break
		}
	}

	if lastImportPos >= 0 {
		insertPos = lastImportPos + 1
	}

	// Insert the import
	newLines := append(lines[:insertPos], append([]string{importValue}, lines[insertPos:]...)...)
	return strings.Join(newLines, "\n")
}

func addImportJavaScript(content, importValue string) string {
	// Check if import already exists
	if strings.Contains(content, importValue) {
		return content
	}

	lines := strings.Split(content, "\n")

	// Find position after existing imports
	insertPos := 0
	lastImportPos := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "const ") && strings.Contains(trimmed, "require(") {
			lastImportPos = i
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "//") && lastImportPos >= 0 {
			insertPos = lastImportPos + 1
			break
		}
	}

	if lastImportPos >= 0 {
		insertPos = lastImportPos + 1
	}

	newLines := append(lines[:insertPos], append([]string{importValue}, lines[insertPos:]...)...)
	return strings.Join(newLines, "\n")
}

func addImportJava(content, importValue string) string {
	// Check if import already exists
	if strings.Contains(content, importValue) {
		return content
	}

	lines := strings.Split(content, "\n")

	// Find position after package declaration and existing imports
	insertPos := 0
	lastImportPos := -1
	foundPackage := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			foundPackage = true
			insertPos = i + 1
		} else if strings.HasPrefix(trimmed, "import ") {
			lastImportPos = i
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "//") && foundPackage {
			if lastImportPos >= 0 {
				insertPos = lastImportPos + 1
			}
			break
		}
	}

	// Ensure import statement ends with semicolon
	if !strings.HasSuffix(importValue, ";") {
		importValue += ";"
	}

	newLines := append(lines[:insertPos], append([]string{importValue}, lines[insertPos:]...)...)
	return strings.Join(newLines, "\n")
}

func addImportRust(content, importValue string) string {
	// Check if use statement already exists
	if strings.Contains(content, importValue) {
		return content
	}

	lines := strings.Split(content, "\n")

	// Find position after existing use statements
	insertPos := 0
	lastUsePos := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "use ") {
			lastUsePos = i
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "//") && lastUsePos >= 0 {
			insertPos = lastUsePos + 1
			break
		}
	}

	if lastUsePos >= 0 {
		insertPos = lastUsePos + 1
	}

	newLines := append(lines[:insertPos], append([]string{importValue}, lines[insertPos:]...)...)
	return strings.Join(newLines, "\n")
}

func addImportC(content, importValue string) string {
	// Check if include already exists
	if strings.Contains(content, importValue) {
		return content
	}

	lines := strings.Split(content, "\n")

	// Find position after existing includes
	insertPos := 0
	lastIncludePos := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#include ") {
			lastIncludePos = i
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") && lastIncludePos >= 0 {
			insertPos = lastIncludePos + 1
			break
		}
	}

	if lastIncludePos >= 0 {
		insertPos = lastIncludePos + 1
	}

	// Ensure include has proper format
	if !strings.HasPrefix(importValue, "#include") {
		importValue = "#include " + importValue
	}

	newLines := append(lines[:insertPos], append([]string{importValue}, lines[insertPos:]...)...)
	return strings.Join(newLines, "\n")
}
