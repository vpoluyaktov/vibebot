package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func extractJavaSymbols(tree *sitter.Tree, content []byte, includePrivate bool) []Symbol {
	var symbols []Symbol
	root := tree.RootNode()

	var traverse func(*sitter.Node)
	traverse = func(node *sitter.Node) {
		switch node.Type() {
		case "method_declaration":
			if sym := extractJavaMethod(node, content); sym != nil {
				if includePrivate || sym.IsExported {
					symbols = append(symbols, *sym)
				}
			}
		case "class_declaration":
			if sym := extractJavaClass(node, content); sym != nil {
				if includePrivate || sym.IsExported {
					symbols = append(symbols, *sym)
				}
			}
		case "interface_declaration":
			if sym := extractJavaInterface(node, content); sym != nil {
				if includePrivate || sym.IsExported {
					symbols = append(symbols, *sym)
				}
			}
		case "field_declaration":
			syms := extractJavaFields(node, content, includePrivate)
			symbols = append(symbols, syms...)
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			traverse(node.Child(i))
		}
	}

	traverse(root)
	return symbols
}

func extractJavaMethod(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	params := findChildByType(node, "formal_parameters")
	
	// Check modifiers for public/private
	isExported := isJavaPublic(node, content)

	sig := name
	if params != nil {
		sig += getNodeText(params, content)
	}

	return &Symbol{
		Name:       name,
		Type:       "method",
		Signature:  sig,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: isExported,
	}
}

func extractJavaClass(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	isExported := isJavaPublic(node, content)

	return &Symbol{
		Name:       name,
		Type:       "class",
		Signature:  "class " + name,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: isExported,
	}
}

func extractJavaInterface(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	isExported := isJavaPublic(node, content)

	return &Symbol{
		Name:       name,
		Type:       "interface",
		Signature:  "interface " + name,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: isExported,
	}
}

func extractJavaFields(node *sitter.Node, content []byte, includePrivate bool) []Symbol {
	var symbols []Symbol
	isExported := isJavaPublic(node, content)

	if !includePrivate && !isExported {
		return symbols
	}

	declarators := findChildrenByType(node, "variable_declarator")
	for _, decl := range declarators {
		nameNode := findChildByType(decl, "identifier")
		if nameNode == nil {
			continue
		}

		name := getNodeText(nameNode, content)

		symbols = append(symbols, Symbol{
			Name:       name,
			Type:       "field",
			Signature:  name,
			StartLine:  decl.StartPoint().Row + 1,
			EndLine:    decl.EndPoint().Row + 1,
			IsExported: isExported,
		})
	}

	return symbols
}

func isJavaPublic(node *sitter.Node, content []byte) bool {
	modifiers := findChildByType(node, "modifiers")
	if modifiers == nil {
		return false
	}

	for i := 0; i < int(modifiers.ChildCount()); i++ {
		child := modifiers.Child(i)
		if child.Type() == "public" {
			return true
		}
	}
	return false
}

func extractJavaImports(tree *sitter.Tree, content []byte) []string {
	var imports []string
	root := tree.RootNode()

	var traverse func(*sitter.Node)
	traverse = func(node *sitter.Node) {
		if node.Type() == "import_declaration" {
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
