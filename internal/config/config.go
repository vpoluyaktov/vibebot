package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the application configuration
type Config struct {
	// Telegram
	TelegramToken    string
	TelegramAllowedUsers []int64
	
	// OpenRouter
	OpenRouterAPIKey string
	OpenRouterModel  string
	
	// Workspace
	WorkspaceDir string
	
	// Logging
	LogLevel string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		TelegramToken:        os.Getenv("TELEGRAM_TOKEN"),
		TelegramAllowedUsers: parseAllowedUsers(os.Getenv("TELEGRAM_ALLOWED_USERS")),
		OpenRouterAPIKey:     os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:      getEnvOrDefault("OPENROUTER_MODEL", "anthropic/claude-3.5-sonnet"),
		WorkspaceDir:         getEnvOrDefault("WORKSPACE_DIR", getDefaultWorkspace()),
		LogLevel:             getEnvOrDefault("LOG_LEVEL", "info"),
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

func parseAllowedUsers(value string) []int64 {
	if value == "" {
		return nil // Empty means allow all
	}
	
	parts := strings.Split(value, ",")
	users := make([]int64, 0, len(parts))
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		userID, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			continue // Skip invalid IDs
		}
		
		users = append(users, userID)
	}
	
	return users
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
