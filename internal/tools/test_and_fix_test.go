package tools

import (
	"context"
	"strings"
	"testing"
)

func TestTestAndFix(t *testing.T) {
	tmpDir := t.TempDir()

	registry := NewRegistry()
	RegisterTestAndFix(registry, tmpDir)

	ctx := context.Background()

	t.Run("basic test command", func(t *testing.T) {
		args := map[string]interface{}{
			"test_command": "go test ./...",
		}

		tool, _ := registry.Get("test_and_fix")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Test Command Information") {
			t.Errorf("Expected test command information header")
		}

		if !strings.Contains(result, "go test ./...") {
			t.Errorf("Expected test command in result")
		}

		if !strings.Contains(result, "use the 'exec' tool") {
			t.Errorf("Expected exec tool suggestion")
		}
	})

	t.Run("with custom working directory", func(t *testing.T) {
		args := map[string]interface{}{
			"test_command": "npm test",
			"working_dir":  "frontend",
		}

		tool, _ := registry.Get("test_and_fix")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "npm test") {
			t.Errorf("Expected npm test command")
		}

		if !strings.Contains(result, "frontend") {
			t.Errorf("Expected working directory in result")
		}
	})

	t.Run("missing test_command", func(t *testing.T) {
		args := map[string]interface{}{}

		tool, _ := registry.Get("test_and_fix")
		_, err := tool.Handler(ctx, args)

		if err == nil {
			t.Error("Expected error for missing test_command")
		}
	})

	t.Run("various test commands", func(t *testing.T) {
		testCases := []string{
			"go test -v ./internal/...",
			"python -m pytest",
			"cargo test",
			"mvn test",
		}

		for _, cmd := range testCases {
			args := map[string]interface{}{
				"test_command": cmd,
			}

			tool, _ := registry.Get("test_and_fix")
			result, err := tool.Handler(ctx, args)

			if err != nil {
				t.Fatalf("Expected no error for command '%s', got: %v", cmd, err)
			}

			if !strings.Contains(result, cmd) {
				t.Errorf("Expected command '%s' in result", cmd)
			}
		}
	})
}
