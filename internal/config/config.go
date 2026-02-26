package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	// Telegram
	TelegramToken string
	
	// OpenRouter
	OpenRouterAPIKey string
	OpenRouterModel  string
	
	// Workspace
	WorkspaceDir string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		TelegramToken:    os.Getenv("TELEGRAM_TOKEN"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:  getEnvOrDefault("OPENROUTER_MODEL", "anthropic/claude-3.5-sonnet"),
		WorkspaceDir:     getEnvOrDefault("WORKSPACE_DIR", getDefaultWorkspace()),
	}

	// Validate required fields
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN is required")
	}
	if cfg.OpenRouterAPIKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY is required")
	}

	// Ensure workspace directory exists
	if err := os.MkdirAll(cfg.WorkspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDefaultWorkspace() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./workspace"
	}
	return filepath.Join(home, ".vibebot", "workspace")
}
