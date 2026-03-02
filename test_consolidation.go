package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/vpoluyaktov/vibebot/internal/config"
	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/memory"
)

func loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

func main() {
	// Load .env file
	if err := loadEnvFile(".env"); err != nil {
		fmt.Printf("Warning: Failed to load .env file: %v\n", err)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize memory
	mem, err := memory.New(cfg.WorkspaceDir)
	if err != nil {
		fmt.Printf("Failed to initialize memory: %v\n", err)
		os.Exit(1)
	}

	// Initialize LLM provider
	provider := llm.NewOpenRouter(cfg.OpenRouterAPIKey, "anthropic/claude-sonnet-4.5")

	// Initialize consolidator
	consolidator := memory.NewConsolidator(mem, provider)

	// Get stats before consolidation
	fmt.Println("=== Before Consolidation ===")
	stats, err := consolidator.GetConsolidationStats("vibebot")
	if err != nil {
		fmt.Printf("Failed to get stats: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Size: %d bytes (~%d tokens)\n", stats["size_bytes"], stats["estimated_tokens"])
	fmt.Printf("Consolidated sections: %d\n", stats["consolidated_sections"])
	fmt.Printf("Total facts: %d\n", stats["total_facts"])
	fmt.Printf("Needs consolidation: %v\n\n", stats["needs_consolidation"])

	// Perform consolidation
	fmt.Println("=== Consolidating ===")
	ctx := context.Background()
	if err := consolidator.ConsolidateProject(ctx, "vibebot"); err != nil {
		fmt.Printf("Consolidation failed: %v\n", err)
		os.Exit(1)
	}

	// Get stats after consolidation
	fmt.Println("\n=== After Consolidation ===")
	newStats, err := consolidator.GetConsolidationStats("vibebot")
	if err != nil {
		fmt.Printf("Failed to get stats: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Size: %d bytes (~%d tokens)\n", newStats["size_bytes"], newStats["estimated_tokens"])
	fmt.Printf("Consolidated sections: %d\n", newStats["consolidated_sections"])
	fmt.Printf("Total facts: %d\n", newStats["total_facts"])

	oldSize := stats["size_bytes"].(int)
	newSize := newStats["size_bytes"].(int)
	reduction := float64(oldSize-newSize) / float64(oldSize) * 100

	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Reduction: %d -> %d bytes (%.1f%%)\n", oldSize, newSize, reduction)
	fmt.Printf("Token savings per request: ~%d tokens\n", (oldSize-newSize)/4)
}
