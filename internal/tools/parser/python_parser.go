package parser

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func extractPythonSymbols(tree *sitter.Tree, content []byte, includePrivate bool) []Symbol {
	var symbols []Symbol
	root := tree.RootNode()

	var traverse func(*sitter.Node)
	traverse = func(node *sitter.Node) {
		switch node.Type() {
		case "function_definition":
			if sym := extractPythonFunction(node, content); sym != nil {
				if includePrivate || sym.IsExported {
					symbols = append(symbols, *sym)
				}
			}
		case "class_definition":
			if sym := extractPythonClass(node, content); sym != nil {
				if includePrivate || sym.IsExported {
					symbols = append(symbols, *sym)
				}
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			traverse(node.Child(i))
		}
	}

	traverse(root)
	return symbols
}

func extractPythonFunction(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	params := findChildByType(node, "parameters")

	sig := "def " + name
	if params != nil {
		sig += getNodeText(params, content)
	}

	// Check if it's a private function (starts with _)
	isExported := !strings.HasPrefix(name, "_")

	return &Symbol{
		Name:       name,
		Type:       "function",
		Signature:  sig,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: isExported,
	}
}

func extractPythonClass(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	
	// Check if it's a private class (starts with _)
	isExported := !strings.HasPrefix(name, "_")

	return &Symbol{
		Name:       name,
		Type:       "class",
		Signature:  "class " + name,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: isExported,
	}
}

func extractPythonImports(tree *sitter.Tree, content []byte) []string {
	var imports []string
	root := tree.RootNode()

	var traverse func(*sitter.Node)
	traverse = func(node *sitter.Node) {
		if node.Type() == "import_statement" || node.Type() == "import_from_statement" {
			importText := getNodeText(node, content)
			imports = append(imports, importText)
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			traverse(node.Child(i))
		}
	}

	traverse(root)
	return imports
}
