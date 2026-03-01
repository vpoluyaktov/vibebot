package tools

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
)

func RegisterFileSummary(registry *Registry, workspaceDir string) {
	registry.Register("file_summary", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "file_summary",
				Description: "Get high-level file metadata without reading full contents. Returns imports, exported symbols, function signatures, and type definitions. Saves tokens by providing structure instead of full code.",
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
						"include_comments": map[string]interface{}{
							"type":        "boolean",
							"description": "Include doc comments for symbols (default: true)",
							"default":     true,
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

			includeComments := true
			if comm, ok := args["include_comments"].(bool); ok {
				includeComments = comm
			}

			// Resolve path
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspaceDir, path)
			}

			// Check if file exists
			if _, err := os.Stat(path); err != nil {
				return "", fmt.Errorf("file not found: %w", err)
			}

			// Determine file type and generate summary
			ext := filepath.Ext(path)
			var summary string
			var err error

			switch ext {
			case ".go":
				summary, err = summarizeGoFile(path, includePrivate, includeComments)
			case ".py":
				summary, err = summarizePythonFile(path, includePrivate, includeComments)
			case ".js", ".ts":
				summary, err = summarizeJavaScriptFile(path, includePrivate, includeComments)
			default:
				summary, err = summarizeGenericFile(path)
			}

			if err != nil {
				return "", fmt.Errorf("failed to summarize file: %w", err)
			}

			logger.Debug("file_summary: summarized %s (%s)", path, ext)
			return summary, nil
		},
	})
}

func summarizeGoFile(path string, includePrivate, includeComments bool) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("File: %s\n", filepath.Base(path)))
	output.WriteString(fmt.Sprintf("Package: %s\n\n", node.Name.Name))

	// Imports
	if len(node.Imports) > 0 {
		output.WriteString("Imports:\n")
		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, "\"")
			if imp.Name != nil {
				output.WriteString(fmt.Sprintf("  %s %s\n", imp.Name.Name, importPath))
			} else {
				output.WriteString(fmt.Sprintf("  %s\n", importPath))
			}
		}
		output.WriteString("\n")
	}

	// Types, Functions, etc.
	var types, funcs, vars, consts []string

	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if ast.IsExported(s.Name.Name) || includePrivate {
						typeDef := formatTypeSpec(s, d.Doc, includeComments)
						types = append(types, typeDef)
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if ast.IsExported(name.Name) || includePrivate {
							varDef := formatValueSpec(name, s, d.Tok.String(), d.Doc, includeComments)
							if d.Tok == token.CONST {
								consts = append(consts, varDef)
							} else {
								vars = append(vars, varDef)
							}
						}
					}
				}
			}
		case *ast.FuncDecl:
			if d.Name != nil && (ast.IsExported(d.Name.Name) || includePrivate) {
				funcDef := formatFuncDecl(d, includeComments)
				funcs = append(funcs, funcDef)
			}
		}
	}

	// Output sections
	if len(types) > 0 {
		output.WriteString("Types:\n")
		for _, t := range types {
			output.WriteString(t)
		}
		output.WriteString("\n")
	}

	if len(consts) > 0 {
		output.WriteString("Constants:\n")
		for _, c := range consts {
			output.WriteString(c)
		}
		output.WriteString("\n")
	}

	if len(vars) > 0 {
		output.WriteString("Variables:\n")
		for _, v := range vars {
			output.WriteString(v)
		}
		output.WriteString("\n")
	}

	if len(funcs) > 0 {
		output.WriteString("Functions:\n")
		for _, f := range funcs {
			output.WriteString(f)
		}
	}

	return output.String(), nil
}

func formatTypeSpec(spec *ast.TypeSpec, doc *ast.CommentGroup, includeComments bool) string {
	var output strings.Builder

	if includeComments && doc != nil {
		output.WriteString(fmt.Sprintf("  // %s\n", strings.TrimSpace(doc.Text())))
	}

	switch t := spec.Type.(type) {
	case *ast.StructType:
		output.WriteString(fmt.Sprintf("  type %s struct { ", spec.Name.Name))
		if t.Fields != nil && len(t.Fields.List) > 0 {
			output.WriteString(fmt.Sprintf("%d fields ", len(t.Fields.List)))
		}
		output.WriteString("}\n")
	case *ast.InterfaceType:
		output.WriteString(fmt.Sprintf("  type %s interface { ", spec.Name.Name))
		if t.Methods != nil && len(t.Methods.List) > 0 {
			output.WriteString(fmt.Sprintf("%d methods ", len(t.Methods.List)))
		}
		output.WriteString("}\n")
	default:
		output.WriteString(fmt.Sprintf("  type %s ...\n", spec.Name.Name))
	}

	return output.String()
}

func formatValueSpec(name *ast.Ident, spec *ast.ValueSpec, kind string, doc *ast.CommentGroup, includeComments bool) string {
	var output strings.Builder

	if includeComments && doc != nil {
		output.WriteString(fmt.Sprintf("  // %s\n", strings.TrimSpace(doc.Text())))
	}

	typeName := ""
	if spec.Type != nil {
		typeName = fmt.Sprintf("%v", spec.Type)
	}

	keyword := strings.ToLower(kind)
	output.WriteString(fmt.Sprintf("  %s %s", keyword, name.Name))
	if typeName != "" {
		output.WriteString(fmt.Sprintf(" %s", typeName))
	}
	if len(spec.Values) > 0 {
		output.WriteString(" = ...")
	}
	output.WriteString("\n")

	return output.String()
}

func formatFuncDecl(decl *ast.FuncDecl, includeComments bool) string {
	var output strings.Builder

	if includeComments && decl.Doc != nil {
		output.WriteString(fmt.Sprintf("  // %s\n", strings.TrimSpace(decl.Doc.Text())))
	}

	// Build function signature
	sig := "  func "
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		recv := decl.Recv.List[0]
		recvType := formatType(recv.Type)
		sig += fmt.Sprintf("(%s) ", recvType)
	}
	sig += decl.Name.Name + "("

	// Parameters
	if decl.Type.Params != nil {
		params := []string{}
		for _, param := range decl.Type.Params.List {
			paramType := formatType(param.Type)
			if len(param.Names) > 0 {
				for _, name := range param.Names {
					params = append(params, name.Name+" "+paramType)
				}
			} else {
				params = append(params, paramType)
			}
		}
		sig += strings.Join(params, ", ")
	}
	sig += ")"

	// Results
	if decl.Type.Results != nil && len(decl.Type.Results.List) > 0 {
		results := []string{}
		for _, result := range decl.Type.Results.List {
			resultType := formatType(result.Type)
			results = append(results, resultType)
		}
		if len(results) == 1 {
			sig += " " + results[0]
		} else {
			sig += " (" + strings.Join(results, ", ") + ")"
		}
	}

	output.WriteString(sig + "\n")
	return output.String()
}

func formatType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + formatType(t.X)
	case *ast.ArrayType:
		return "[]" + formatType(t.Elt)
	case *ast.MapType:
		return "map[" + formatType(t.Key) + "]" + formatType(t.Value)
	case *ast.SelectorExpr:
		return formatType(t.X) + "." + t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func summarizePythonFile(path string, includePrivate, includeComments bool) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	var output strings.Builder
	output.WriteString(fmt.Sprintf("File: %s\n\n", filepath.Base(path)))

	// Imports
	var imports []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") {
			imports = append(imports, trimmed)
		}
	}
	if len(imports) > 0 {
		output.WriteString("Imports:\n")
		for _, imp := range imports {
			output.WriteString(fmt.Sprintf("  %s\n", imp))
		}
		output.WriteString("\n")
	}

	// Classes and functions (simple regex-like parsing)
	var classes, functions []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "class ") {
			className := extractPythonName(trimmed, "class ")
			if includePrivate || !strings.HasPrefix(className, "_") {
				classes = append(classes, fmt.Sprintf("  class %s", className))
			}
		} else if strings.HasPrefix(trimmed, "def ") {
			funcName := extractPythonName(trimmed, "def ")
			if includePrivate || !strings.HasPrefix(funcName, "_") {
				comment := ""
				if includeComments && i > 0 {
					prevLine := strings.TrimSpace(lines[i-1])
					if strings.HasPrefix(prevLine, "#") {
						comment = " " + prevLine
					}
				}
				functions = append(functions, fmt.Sprintf("  def %s%s", funcName, comment))
			}
		}
	}

	if len(classes) > 0 {
		output.WriteString("Classes:\n")
		for _, c := range classes {
			output.WriteString(c + "\n")
		}
		output.WriteString("\n")
	}

	if len(functions) > 0 {
		output.WriteString("Functions:\n")
		for _, f := range functions {
			output.WriteString(f + "\n")
		}
	}

	return output.String(), nil
}

func extractPythonName(line, prefix string) string {
	line = strings.TrimPrefix(line, prefix)
	if idx := strings.Index(line, "("); idx != -1 {
		return strings.TrimSpace(line[:idx])
	}
	if idx := strings.Index(line, ":"); idx != -1 {
		return strings.TrimSpace(line[:idx])
	}
	return strings.TrimSpace(line)
}

func summarizeJavaScriptFile(path string, includePrivate, includeComments bool) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	var output strings.Builder
	output.WriteString(fmt.Sprintf("File: %s\n\n", filepath.Base(path)))

	// Imports/requires
	var imports []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "export ") || strings.Contains(trimmed, "require(") {
			imports = append(imports, trimmed)
		}
	}
	if len(imports) > 0 {
		output.WriteString("Imports:\n")
		for _, imp := range imports {
			output.WriteString(fmt.Sprintf("  %s\n", imp))
		}
		output.WriteString("\n")
	}

	// Functions and classes
	var functions, classes []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "function ") || strings.HasPrefix(trimmed, "export function ") {
			functions = append(functions, fmt.Sprintf("  %s", trimmed))
		} else if strings.HasPrefix(trimmed, "class ") || strings.HasPrefix(trimmed, "export class ") {
			classes = append(classes, fmt.Sprintf("  %s", trimmed))
		} else if strings.Contains(trimmed, "= function") || strings.Contains(trimmed, "=> {") {
			functions = append(functions, fmt.Sprintf("  %s", trimmed))
		}
	}

	if len(classes) > 0 {
		output.WriteString("Classes:\n")
		for _, c := range classes {
			output.WriteString(c + "\n")
		}
		output.WriteString("\n")
	}

	if len(functions) > 0 {
		output.WriteString("Functions:\n")
		for _, f := range functions {
			output.WriteString(f + "\n")
		}
	}

	return output.String(), nil
}

func summarizeGenericFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Adjust line count if file doesn't end with newline
	lineCount := len(lines)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		// Split creates an extra element, but file doesn't end with newline
		// so the count is correct
	} else if len(lines) > 0 && lines[len(lines)-1] == "" {
		// File ends with newline, split creates empty last element
		lineCount--
	}

	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}

	return fmt.Sprintf("File: %s\nSize: %d bytes\nLines: %d (non-empty: %d)\nType: %s\n",
		filepath.Base(path), info.Size(), lineCount, nonEmptyLines, filepath.Ext(path)), nil
}
