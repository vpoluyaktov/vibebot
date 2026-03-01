package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
)

func extractCSymbols(tree *sitter.Tree, content []byte, includePrivate bool) []Symbol {
	var symbols []Symbol
	root := tree.RootNode()

	var traverse func(*sitter.Node)
	traverse = func(node *sitter.Node) {
		switch node.Type() {
		case "function_definition":
			if sym := extractCFunction(node, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		case "struct_specifier":
			if sym := extractCStruct(node, content); sym != nil {
				symbols = append(symbols, *sym)
			}
		case "declaration":
			syms := extractCDeclarations(node, content)
			symbols = append(symbols, syms...)
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			traverse(node.Child(i))
		}
	}

	traverse(root)
	return symbols
}

func extractCFunction(node *sitter.Node, content []byte) *Symbol {
	declarator := findChildByType(node, "function_declarator")
	if declarator == nil {
		return nil
	}

	nameNode := findChildByType(declarator, "identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	params := findChildByType(declarator, "parameter_list")

	sig := name
	if params != nil {
		sig += getNodeText(params, content)
	}

	return &Symbol{
		Name:       name,
		Type:       "function",
		Signature:  sig,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: true, // C doesn't have visibility modifiers
	}
}

func extractCStruct(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "type_identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)

	return &Symbol{
		Name:       name,
		Type:       "struct",
		Signature:  "struct " + name,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: true,
	}
}

func extractCDeclarations(node *sitter.Node, content []byte) []Symbol {
	var symbols []Symbol

	declarators := findChildrenByType(node, "init_declarator")
	for _, decl := range declarators {
		nameNode := findChildByType(decl, "identifier")
		if nameNode == nil {
			continue
		}

		name := getNodeText(nameNode, content)

		symbols = append(symbols, Symbol{
			Name:       name,
			Type:       "variable",
			Signature:  name,
			StartLine:  decl.StartPoint().Row + 1,
			EndLine:    decl.EndPoint().Row + 1,
			IsExported: true,
		})
	}

	return symbols
}
