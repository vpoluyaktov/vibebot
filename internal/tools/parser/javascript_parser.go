package parser

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func extractJavaScriptSymbols(tree *sitter.Tree, content []byte, includePrivate bool) []Symbol {
	var symbols []Symbol
	root := tree.RootNode()

	var traverse func(*sitter.Node)
	traverse = func(node *sitter.Node) {
		switch node.Type() {
		case "function_declaration":
			if sym := extractJSFunction(node, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		case "class_declaration":
			if sym := extractJSClass(node, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		case "method_definition":
			if sym := extractJSMethod(node, content); sym != nil {
				if includePrivate || sym.IsExported {
					symbols = append(symbols, *sym)
				}
			}
		case "lexical_declaration", "variable_declaration":
			syms := extractJSVariables(node, content, includePrivate)
			symbols = append(symbols, syms...)
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			traverse(node.Child(i))
		}
	}

	traverse(root)
	return symbols
}

func extractJSFunction(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	params := findChildByType(node, "formal_parameters")

	sig := "function " + name
	if params != nil {
		sig += getNodeText(params, content)
	}

	return &Symbol{
		Name:       name,
		Type:       "function",
		Signature:  sig,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: true, // JS doesn't have built-in export visibility
	}
}

func extractJSClass(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)

	return &Symbol{
		Name:       name,
		Type:       "class",
		Signature:  "class " + name,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: true,
	}
}

func extractJSMethod(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "property_identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	params := findChildByType(node, "formal_parameters")

	sig := name
	if params != nil {
		sig += getNodeText(params, content)
	}

	// Check if it's a private method (starts with #)
	isExported := !strings.HasPrefix(name, "#")

	return &Symbol{
		Name:       name,
		Type:       "method",
		Signature:  sig,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: isExported,
	}
}

func extractJSVariables(node *sitter.Node, content []byte, includePrivate bool) []Symbol {
	var symbols []Symbol

	declarators := findChildrenByType(node, "variable_declarator")
	for _, decl := range declarators {
		nameNode := findChildByType(decl, "identifier")
		if nameNode == nil {
			continue
		}

		name := getNodeText(nameNode, content)
		
		// Determine if it's const or let/var
		typeName := "var"
		if node.Type() == "lexical_declaration" {
			// Check if it's const or let
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "const" {
					typeName = "const"
					break
				} else if child.Type() == "let" {
					typeName = "let"
					break
				}
			}
		}

		symbols = append(symbols, Symbol{
			Name:       name,
			Type:       typeName,
			Signature:  typeName + " " + name,
			StartLine:  decl.StartPoint().Row + 1,
			EndLine:    decl.EndPoint().Row + 1,
			IsExported: true,
		})
	}

	return symbols
}

func extractJavaScriptImports(tree *sitter.Tree, content []byte) []string {
	var imports []string
	root := tree.RootNode()

	var traverse func(*sitter.Node)
	traverse = func(node *sitter.Node) {
		if node.Type() == "import_statement" {
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
