package main

import (
	"fmt"
	"strings"

	"github.com/vpoluyaktov/vibebot/internal/tools"
)

func main() {
	registry := tools.NewRegistry()
	workspaceDir := "/tmp/test"

	// Register all tools
	tools.RegisterFileTools(registry, workspaceDir)
	tools.RegisterExecTool(registry, workspaceDir)
	tools.RegisterBatchTools(registry)
	tools.RegisterMultiFileRead(registry, workspaceDir)
	tools.RegisterSearchAndRead(registry, workspaceDir)
	tools.RegisterCodeContext(registry, workspaceDir)
	tools.RegisterDiffPreview(registry, workspaceDir)
	tools.RegisterWorkspaceSnapshot(registry, workspaceDir)
	tools.RegisterSmartEdit(registry, workspaceDir)
	tools.RegisterTestAndFix(registry, workspaceDir)
	tools.RegisterFileSummary(registry, workspaceDir)
	tools.RegisterFileOutline(registry, workspaceDir)
	tools.RegisterSymbolDefinition(registry, workspaceDir)
	tools.RegisterCachedGrep(registry, workspaceDir)
	tools.RegisterIncrementalEdit(registry, workspaceDir)

	definitions := registry.GetDefinitions()

	fmt.Println("Tool Descriptions Analysis:")
	fmt.Println("===========================\n")

	for _, def := range definitions {
		desc := def.Function.Description
		words := len(strings.Fields(desc))
		chars := len(desc)

		fmt.Printf("%-25s | %3d chars | %2d words\n", def.Function.Name, chars, words)
		fmt.Printf("  Description: %s\n", desc)

		// Check for redundant phrases
		redundant := []string{}
		if strings.Contains(desc, "Use this") {
			redundant = append(redundant, "Use this")
		}
		if strings.Contains(desc, "This tool") {
			redundant = append(redundant, "This tool")
		}
		if strings.Contains(desc, "Allows you to") {
			redundant = append(redundant, "Allows you to")
		}
		if strings.Contains(desc, "Can be used to") {
			redundant = append(redundant, "Can be used to")
		}

		if len(redundant) > 0 {
			fmt.Printf("  ⚠️  Redundant phrases: %v\n", redundant)
		}

		fmt.Println()
	}
}
