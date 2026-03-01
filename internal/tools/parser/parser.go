package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Language represents a programming language
type Language struct {
	Name       string
	Extensions []string
	Parser     *sitter.Language
}

// SupportedLanguages contains all languages we can parse
var SupportedLanguages = []Language{
	{
		Name:       "go",
		Extensions: []string{".go"},
		Parser:     golang.GetLanguage(),
	},
	{
		Name:       "python",
		Extensions: []string{".py"},
		Parser:     python.GetLanguage(),
	},
	{
		Name:       "javascript",
		Extensions: []string{".js", ".jsx", ".mjs"},
		Parser:     javascript.GetLanguage(),
	},
	{
		Name:       "typescript",
		Extensions: []string{".ts", ".tsx"},
		Parser:     typescript.GetLanguage(),
	},
	{
		Name:       "java",
		Extensions: []string{".java"},
		Parser:     java.GetLanguage(),
	},
	{
		Name:       "c",
		Extensions: []string{".c", ".h"},
		Parser:     c.GetLanguage(),
	},
	{
		Name:       "cpp",
		Extensions: []string{".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx"},
		Parser:     cpp.GetLanguage(),
	},
	{
		Name:       "rust",
		Extensions: []string{".rs"},
		Parser:     rust.GetLanguage(),
	},
}

// GetLanguageByExtension returns the language for a file extension
func GetLanguageByExtension(ext string) (*Language, bool) {
	ext = strings.ToLower(ext)
	for i := range SupportedLanguages {
		for _, langExt := range SupportedLanguages[i].Extensions {
			if langExt == ext {
				return &SupportedLanguages[i], true
			}
		}
	}
	return nil, false
}

// GetLanguageByName returns the language by name
func GetLanguageByName(name string) (*Language, bool) {
	name = strings.ToLower(name)
	for i := range SupportedLanguages {
		if SupportedLanguages[i].Name == name {
			return &SupportedLanguages[i], true
		}
	}
	return nil, false
}

// ParseFile parses a file and returns the syntax tree
func ParseFile(path string) (*sitter.Tree, *Language, error) {
	ext := filepath.Ext(path)
	lang, ok := GetLanguageByExtension(ext)
	if !ok {
		return nil, nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang.Parser)

	tree := parser.Parse(nil, content)
	if tree == nil {
		return nil, nil, fmt.Errorf("failed to parse file")
	}

	return tree, lang, nil
}

// Symbol represents a code symbol (function, class, method, etc.)
type Symbol struct {
	Name       string
	Type       string // function, method, class, struct, interface, variable, constant
	Signature  string
	StartLine  uint32
	EndLine    uint32
	DocComment string
	IsExported bool
	Receiver   string // For methods
}

// ExtractSymbols extracts all symbols from a parsed tree
func ExtractSymbols(tree *sitter.Tree, lang *Language, content []byte, includePrivate bool) []Symbol {
	switch lang.Name {
	case "go":
		return extractGoSymbols(tree, content, includePrivate)
	case "python":
		return extractPythonSymbols(tree, content, includePrivate)
	case "javascript", "typescript":
		return extractJavaScriptSymbols(tree, content, includePrivate)
	case "java":
		return extractJavaSymbols(tree, content, includePrivate)
	case "c", "cpp":
		return extractCSymbols(tree, content, includePrivate)
	case "rust":
		return extractRustSymbols(tree, content, includePrivate)
	default:
		return []Symbol{}
	}
}

// Helper function to get node text
func getNodeText(node *sitter.Node, content []byte) string {
	return string(content[node.StartByte():node.EndByte()])
}

// Helper function to check if a name is exported (starts with uppercase)
func isExportedName(name string) bool {
	if len(name) == 0 {
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

// Helper function to find child by type
func findChildByType(node *sitter.Node, childType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == childType {
			return child
		}
	}
	return nil
}

// Helper function to find all children by type
func findChildrenByType(node *sitter.Node, childType string) []*sitter.Node {
	var result []*sitter.Node
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == childType {
			result = append(result, child)
		}
	}
	return result
}

// GetImports extracts import statements from a file
func GetImports(tree *sitter.Tree, lang *Language, content []byte) []string {
	switch lang.Name {
	case "go":
		return extractGoImports(tree, content)
	case "python":
		return extractPythonImports(tree, content)
	case "javascript", "typescript":
		return extractJavaScriptImports(tree, content)
	case "java":
		return extractJavaImports(tree, content)
	default:
		return []string{}
	}
}
