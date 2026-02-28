package tools

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
)

func RegisterTestAndFix(registry *Registry, workspaceDir string) {
	registry.Register("test_and_fix", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "test_and_fix",
				Description: "Run tests and report results. Useful for verifying changes work correctly. Note: This tool provides test command information; use 'exec' tool to actually run tests.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"test_command": map[string]interface{}{
							"type":        "string",
							"description": "Test command to run (e.g., 'go test ./...', 'npm test')",
						},
						"working_dir": map[string]interface{}{
							"type":        "string",
							"description": "Working directory for test command (default: workspace root)",
							"default":     ".",
						},
					},
					"required": []string{"test_command"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			testCommand, ok := args["test_command"].(string)
			if !ok {
				return "", fmt.Errorf("test_command must be a string")
			}

			workingDir := workspaceDir
			if dir, ok := args["working_dir"].(string); ok && dir != "." {
				if !filepath.IsAbs(dir) {
					workingDir = filepath.Join(workspaceDir, dir)
				} else {
					workingDir = dir
				}
			}

			logger.Debug("test_and_fix: would run '%s' in %s", testCommand, workingDir)

			// Return information about how to run the test
			return fmt.Sprintf("Test Command Information:\n\nCommand: %s\nWorking Directory: %s\n\nTo execute this test, use the 'exec' tool with:\n{\n  \"command\": \"%s\"\n}\n\nOr use batch_tools to run the test along with other operations.", testCommand, workingDir, testCommand), nil
		},
	})
}
