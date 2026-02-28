package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSmartEdit(t *testing.T) {
	tmpDir := t.TempDir()

	registry := NewRegistry()
	RegisterSmartEdit(registry, tmpDir)

	ctx := context.Background()

	t.Run("add_import to file with import block", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test1.go")
		os.WriteFile(testFile, []byte("package main\n\nimport (\n\t\"fmt\"\n)\n\nfunc main() {}"), 0644)

		args := map[string]interface{}{
			"path":      "test1.go",
			"operation": "add_import",
			"value":     "os",
		}

		tool, _ := registry.Get("smart_edit")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Successfully performed add_import") {
			t.Errorf("Expected success message, got: %s", result)
		}

		// Verify file was modified
		data, _ := os.ReadFile(testFile)
		content := string(data)

		if !strings.Contains(content, "\"os\"") {
			t.Errorf("Expected os import to be added, got: %s", content)
		}
	})

	t.Run("add_import to file without import block", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test2.go")
		os.WriteFile(testFile, []byte("package main\n\nfunc main() {}"), 0644)

		args := map[string]interface{}{
			"path":      "test2.go",
			"operation": "add_import",
			"value":     "fmt",
		}

		tool, _ := registry.Get("smart_edit")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Successfully performed add_import") {
			t.Errorf("Expected success message, got: %s", result)
		}

		// Verify file was modified
		data, _ := os.ReadFile(testFile)
		content := string(data)

		if !strings.Contains(content, "import \"fmt\"") {
			t.Errorf("Expected fmt import to be added, got: %s", content)
		}
	})

	t.Run("add_function", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test3.go")
		os.WriteFile(testFile, []byte("package main\n\nfunc main() {}"), 0644)

		args := map[string]interface{}{
			"path":      "test3.go",
			"operation": "add_function",
			"value":     "func Helper() {\n\t// helper code\n}",
		}

		tool, _ := registry.Get("smart_edit")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Successfully performed add_function") {
			t.Errorf("Expected success message, got: %s", result)
		}

		// Verify file was modified
		data, _ := os.ReadFile(testFile)
		content := string(data)

		if !strings.Contains(content, "func Helper()") {
			t.Errorf("Expected Helper function to be added, got: %s", content)
		}
	})

	t.Run("append_content", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test4.txt")
		os.WriteFile(testFile, []byte("Line 1\nLine 2"), 0644)

		args := map[string]interface{}{
			"path":      "test4.txt",
			"operation": "append_content",
			"value":     "Line 3",
		}

		tool, _ := registry.Get("smart_edit")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Successfully performed append_content") {
			t.Errorf("Expected success message, got: %s", result)
		}

		// Verify file was modified
		data, _ := os.ReadFile(testFile)
		content := string(data)

		if !strings.Contains(content, "Line 3") {
			t.Errorf("Expected Line 3 to be appended, got: %s", content)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		args := map[string]interface{}{
			"path":      "nonexistent.go",
			"operation": "add_function",
			"value":     "func Test() {}",
		}

		tool, _ := registry.Get("smart_edit")
		_, err := tool.Handler(ctx, args)

		if err == nil {
			t.Error("Expected error for missing file")
		}
	})

	t.Run("unsupported operation", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test5.go")
		os.WriteFile(testFile, []byte("package main"), 0644)

		args := map[string]interface{}{
			"path":      "test5.go",
			"operation": "invalid_operation",
			"value":     "something",
		}

		tool, _ := registry.Get("smart_edit")
		_, err := tool.Handler(ctx, args)

		if err == nil {
			t.Error("Expected error for unsupported operation")
		}
	})

	t.Run("missing required parameters", func(t *testing.T) {
		args := map[string]interface{}{
			"path": "test.go",
		}

		tool, _ := registry.Get("smart_edit")
		_, err := tool.Handler(ctx, args)

		if err == nil {
			t.Error("Expected error for missing parameters")
		}
	})
}
