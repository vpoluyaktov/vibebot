package parser

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func extractGoSymbols(tree *sitter.Tree, content []byte, includePrivate bool) []Symbol {
	var symbols []Symbol
	root := tree.RootNode()

	// Traverse the tree
	var traverse func(*sitter.Node)
	traverse = func(node *sitter.Node) {
		switch node.Type() {
		case "function_declaration":
			if sym := extractGoFunction(node, content); sym != nil {
				if includePrivate || sym.IsExported {
					symbols = append(symbols, *sym)
				}
			}
		case "method_declaration":
			if sym := extractGoMethod(node, content); sym != nil {
				if includePrivate || sym.IsExported {
					symbols = append(symbols, *sym)
				}
			}
		case "type_declaration":
			syms := extractGoTypes(node, content, includePrivate)
			symbols = append(symbols, syms...)
		case "const_declaration", "var_declaration":
			syms := extractGoVars(node, content, includePrivate)
			symbols = append(symbols, syms...)
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			traverse(node.Child(i))
		}
	}

	traverse(root)
	return symbols
}

func extractGoFunction(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	params := findChildByType(node, "parameter_list")
	result := findChildByType(node, "result")

	sig := "func " + name
	if params != nil {
		sig += getNodeText(params, content)
	}
	if result != nil {
		sig += " " + getNodeText(result, content)
	}

	return &Symbol{
		Name:       name,
		Type:       "function",
		Signature:  sig,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: isExportedName(name),
	}
}

func extractGoMethod(node *sitter.Node, content []byte) *Symbol {
	nameNode := findChildByType(node, "field_identifier")
	if nameNode == nil {
		return nil
	}

	name := getNodeText(nameNode, content)
	receiver := findChildByType(node, "parameter_list")
	params := node.ChildByFieldName("parameters")
	result := node.ChildByFieldName("result")

	var receiverType string
	if receiver != nil {
		receiverType = getNodeText(receiver, content)
	}

	sig := "func " + receiverType + " " + name
	if params != nil {
		sig += getNodeText(params, content)
	}
	if result != nil {
		sig += " " + getNodeText(result, content)
	}

	return &Symbol{
		Name:       name,
		Type:       "method",
		Signature:  sig,
		StartLine:  node.StartPoint().Row + 1,
		EndLine:    node.EndPoint().Row + 1,
		IsExported: isExportedName(name),
		Receiver:   receiverType,
	}
}

func extractGoTypes(node *sitter.Node, content []byte, includePrivate bool) []Symbol {
	var symbols []Symbol

	specs := findChildrenByType(node, "type_spec")
	for _, spec := range specs {
		nameNode := findChildByType(spec, "type_identifier")
		if nameNode == nil {
			continue
		}

		name := getNodeText(nameNode, content)
		if !includePrivate && !isExportedName(name) {
			continue
		}

		typeNode := spec.ChildByFieldName("type")
		var typeName string
		if typeNode != nil {
			switch typeNode.Type() {
			case "struct_type":
				typeName = "struct"
			case "interface_type":
				typeName = "interface"
			default:
				typeName = "type"
			}
		}

		symbols = append(symbols, Symbol{
			Name:       name,
			Type:       typeName,
			Signature:  "type " + name + " " + typeName,
			StartLine:  spec.StartPoint().Row + 1,
			EndLine:    spec.EndPoint().Row + 1,
			IsExported: isExportedName(name),
		})
	}

	return symbols
}

func extractGoVars(node *sitter.Node, content []byte, includePrivate bool) []Symbol {
	var symbols []Symbol
	isConst := node.Type() == "const_declaration"

	specs := findChildrenByType(node, "const_spec")
	if !isConst {
		specs = findChildrenByType(node, "var_spec")
	}

	for _, spec := range specs {
		nameNode := findChildByType(spec, "identifier")
		if nameNode == nil {
			continue
		}

		name := getNodeText(nameNode, content)
		if !includePrivate && !isExportedName(name) {
			continue
		}

		typeName := "var"
		if isConst {
			typeName = "const"
		}

		symbols = append(symbols, Symbol{
			Name:       name,
			Type:       typeName,
			Signature:  typeName + " " + name,
			StartLine:  spec.StartPoint().Row + 1,
			EndLine:    spec.EndPoint().Row + 1,
			IsExported: isExportedName(name),
		})
	}

	return symbols
}

func extractGoImports(tree *sitter.Tree, content []byte) []string {
	var imports []string
	root := tree.RootNode()

	var traverse func(*sitter.Node)
	traverse = func(node *sitter.Node) {
		if node.Type() == "import_declaration" {
			// Look for import_spec or import_spec_list
			for i := 0; i < int(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Type() == "import_spec" {
					pathNode := findChildByType(child, "interpreted_string_literal")
					if pathNode != nil {
						importPath := getNodeText(pathNode, content)
						importPath = strings.Trim(importPath, "\"")
						imports = append(imports, importPath)
					}
				} else if child.Type() == "import_spec_list" {
					// Handle import block
					for j := 0; j < int(child.ChildCount()); j++ {
						spec := child.Child(j)
						if spec.Type() == "import_spec" {
							pathNode := findChildByType(spec, "interpreted_string_literal")
							if pathNode != nil {
								importPath := getNodeText(pathNode, content)
								importPath = strings.Trim(importPath, "\"")
								imports = append(imports, importPath)
							}
						}
					}
				}
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			traverse(node.Child(i))
		}
	}

	traverse(root)
	return imports
}
