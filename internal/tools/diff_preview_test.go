package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffPreview(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "config.go"), []byte("port := 8080\nhost := localhost"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "server.go"), []byte("const Port = 8080"), 0644)

	registry := NewRegistry()
	RegisterDiffPreview(registry, tmpDir)

	ctx := context.Background()

	t.Run("preview single edit", func(t *testing.T) {
		args := map[string]interface{}{
			"edits": []interface{}{
				map[string]interface{}{
					"path":     "config.go",
					"old_text": "port := 8080",
					"new_text": "port := 9000",
				},
			},
		}

		tool, _ := registry.Get("diff_preview")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Preview of 1/1 edits") {
			t.Errorf("Expected preview message, got: %s", result)
		}

		if !strings.Contains(result, "--- Before:") {
			t.Errorf("Expected before section")
		}

		if !strings.Contains(result, "+++ After:") {
			t.Errorf("Expected after section")
		}

		if !strings.Contains(result, "port := 8080") {
			t.Errorf("Expected old text in preview")
		}

		if !strings.Contains(result, "port := 9000") {
			t.Errorf("Expected new text in preview")
		}
	})

	t.Run("preview multiple edits", func(t *testing.T) {
		args := map[string]interface{}{
			"edits": []interface{}{
				map[string]interface{}{
					"path":     "config.go",
					"old_text": "8080",
					"new_text": "9000",
				},
				map[string]interface{}{
					"path":     "server.go",
					"old_text": "8080",
					"new_text": "9000",
				},
			},
		}

		tool, _ := registry.Get("diff_preview")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Preview of 2/2 edits") {
			t.Errorf("Expected 2/2 preview message, got: %s", result)
		}
	})

	t.Run("old_text not found", func(t *testing.T) {
		args := map[string]interface{}{
			"edits": []interface{}{
				map[string]interface{}{
					"path":     "config.go",
					"old_text": "nonexistent text",
					"new_text": "replacement",
				},
			},
		}

		tool, _ := registry.Get("diff_preview")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "old_text not found") {
			t.Errorf("Expected not found error, got: %s", result)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		args := map[string]interface{}{
			"edits": []interface{}{
				map[string]interface{}{
					"path":     "nonexistent.go",
					"old_text": "text",
					"new_text": "replacement",
				},
			},
		}

		tool, _ := registry.Get("diff_preview")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "ERROR") {
			t.Errorf("Expected error for missing file, got: %s", result)
		}
	})

	t.Run("partial success", func(t *testing.T) {
		args := map[string]interface{}{
			"edits": []interface{}{
				map[string]interface{}{
					"path":     "config.go",
					"old_text": "8080",
					"new_text": "9000",
				},
				map[string]interface{}{
					"path":     "nonexistent.go",
					"old_text": "text",
					"new_text": "replacement",
				},
			},
		}

		tool, _ := registry.Get("diff_preview")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Preview of 1/2 edits") {
			t.Errorf("Expected 1/2 preview message, got: %s", result)
		}
	})

	t.Run("empty edits array", func(t *testing.T) {
		args := map[string]interface{}{
			"edits": []interface{}{},
		}

		tool, _ := registry.Get("diff_preview")
		_, err := tool.Handler(ctx, args)

		if err == nil {
			t.Error("Expected error for empty edits array")
		}
	})

	t.Run("missing edits parameter", func(t *testing.T) {
		args := map[string]interface{}{}

		tool, _ := registry.Get("diff_preview")
		_, err := tool.Handler(ctx, args)

		if err == nil {
			t.Error("Expected error for missing edits parameter")
		}
	})

	t.Run("truncate long text", func(t *testing.T) {
		longText := strings.Repeat("a", 300)
		os.WriteFile(filepath.Join(tmpDir, "long.txt"), []byte(longText), 0644)

		args := map[string]interface{}{
			"edits": []interface{}{
				map[string]interface{}{
					"path":     "long.txt",
					"old_text": longText,
					"new_text": "short",
				},
			},
		}

		tool, _ := registry.Get("diff_preview")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "(truncated)") {
			t.Errorf("Expected truncation message for long text")
		}
	})
}
