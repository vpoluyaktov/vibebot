package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/vpoluyaktov/vibebot/internal/tools"
)

func main() {
	// Create a registry and register all tools (simulating main.go)
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
	
	// Get all tool definitions
	definitions := registry.GetDefinitions()
	
	// Convert to JSON
	jsonData, err := json.MarshalIndent(definitions, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	
	// Calculate size
	jsonSize := len(jsonData)
	estimatedTokens := jsonSize / 4 // Rough estimate: 1 token ≈ 4 chars
	
	fmt.Printf("Tool Definitions Analysis:\n")
	fmt.Printf("========================\n")
	fmt.Printf("Number of tools: %d\n", len(definitions))
	fmt.Printf("JSON size: %d bytes\n", jsonSize)
	fmt.Printf("Estimated tokens: ~%d tokens\n", estimatedTokens)
	fmt.Printf("\nPer-tool average: ~%d tokens\n", estimatedTokens/len(definitions))
	
	// List all tools
	fmt.Printf("\nRegistered tools:\n")
	for i, def := range definitions {
		toolJSON, _ := json.Marshal(def)
		fmt.Printf("%2d. %-30s (~%d tokens)\n", i+1, def.Function.Name, len(toolJSON)/4)
	}
}
