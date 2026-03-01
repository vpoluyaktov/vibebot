package parser

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func extractRustSymbols(tree *sitter.Tree, content []byte, includePrivate bool) []Symbol {
	var symbols []Symbol
	root := tree.RootNode()

	var traverse func(*sitter.Node)
	traverse = func(node *sitter.Node) {
		switch node.Type() {
		case "function_item":
			if sym := extractRustFunction(node, content); sym != nil {
				if includePrivate || sym.IsExported {
					symbols = append(symbols, *sym)
				}
			}
		case "struct_item":
			if sym := extractRustStruct(node, content); sym != nil {
				if includePrivate || sym.IsExported {
					symbols = append(symbols, *sym)
				}
			}
		case "impl_item":
			syms := extractRustImpl(node, content, includePrivate)
			symbols = append(symbols, syms...)
		case "trait_item":
			if sym := extractRustTrait(node, content); sym != nil {
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

func extractRustFunction(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	params := findChildByType(node, "parameters")
	
	// Check for pub modifier
	isExported := isRustPublic(node, content)

	sig := "fn " + name
	if params != nil {
		sig += getNodeText(params, content)
	}

	return &Symbol{
		Name:       name,
		Type:       "function",
		Signature:  sig,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: isExported,
	}
}

func extractRustStruct(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "type_identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	isExported := isRustPublic(node, content)

	return &Symbol{
		Name:       name,
		Type:       "struct",
		Signature:  "struct " + name,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: isExported,
	}
}

func extractRustTrait(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "type_identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	isExported := isRustPublic(node, content)

	return &Symbol{
		Name:       name,
		Type:       "trait",
		Signature:  "trait " + name,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: isExported,
	}
}

func extractRustImpl(node *sitter.Node, content []byte, includePrivate bool) []Symbol {
	var symbols []Symbol

	// Find all function items in the impl block
	var traverse func(*sitter.Node)
	traverse = func(n *sitter.Node) {
		if n.Type() == "function_item" {
			if sym := extractRustFunction(n, content); sym != nil {
				if includePrivate || sym.IsExported {
					sym.Type = "method"
					symbols = append(symbols, *sym)
				}
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			traverse(n.Child(i))
		}
	}

	traverse(node)
	return symbols
}

func isRustPublic(node *sitter.Node, content []byte) bool {
	// Look for visibility_modifier child
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "visibility_modifier" {
			text := getNodeText(child, content)
			return strings.Contains(text, "pub")
		}
	}
	return false
}
