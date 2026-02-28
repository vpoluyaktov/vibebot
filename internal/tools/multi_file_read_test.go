package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMultiFileRead(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	file3 := filepath.Join(tmpDir, "file3.txt")

	os.WriteFile(file1, []byte("Content of file 1"), 0644)
	os.WriteFile(file2, []byte("Content of file 2"), 0644)
	os.WriteFile(file3, []byte("Content of file 3"), 0644)

	registry := NewRegistry()
	RegisterMultiFileRead(registry, tmpDir)

	ctx := context.Background()

	t.Run("read multiple files successfully", func(t *testing.T) {
		args := map[string]interface{}{
			"paths": []interface{}{"file1.txt", "file2.txt", "file3.txt"},
		}

		tool, _ := registry.Get("multi_file_read")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Read 3/3 files successfully") {
			t.Errorf("Expected 3/3 success message, got: %s", result)
		}

		if !strings.Contains(result, "Content of file 1") {
			t.Errorf("Expected file1 content in result")
		}

		if !strings.Contains(result, "Content of file 2") {
			t.Errorf("Expected file2 content in result")
		}

		if !strings.Contains(result, "Content of file 3") {
			t.Errorf("Expected file3 content in result")
		}
	})

	t.Run("read with metadata", func(t *testing.T) {
		args := map[string]interface{}{
			"paths":            []interface{}{"file1.txt"},
			"include_metadata": true,
		}

		tool, _ := registry.Get("multi_file_read")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Size:") {
			t.Errorf("Expected metadata with size in result")
		}

		if !strings.Contains(result, "Modified:") {
			t.Errorf("Expected metadata with modified time in result")
		}
	})

	t.Run("handle missing file", func(t *testing.T) {
		args := map[string]interface{}{
			"paths": []interface{}{"file1.txt", "nonexistent.txt", "file2.txt"},
		}

		tool, _ := registry.Get("multi_file_read")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Read 2/3 files successfully") {
			t.Errorf("Expected 2/3 success message, got: %s", result)
		}

		if !strings.Contains(result, "ERROR") {
			t.Errorf("Expected ERROR for missing file")
		}
	})

	t.Run("empty paths array", func(t *testing.T) {
		args := map[string]interface{}{
			"paths": []interface{}{},
		}

		tool, _ := registry.Get("multi_file_read")
		_, err := tool.Handler(ctx, args)

		if err == nil {
			t.Error("Expected error for empty paths array")
		}
	})

	t.Run("missing paths parameter", func(t *testing.T) {
		args := map[string]interface{}{}

		tool, _ := registry.Get("multi_file_read")
		_, err := tool.Handler(ctx, args)

		if err == nil {
			t.Error("Expected error for missing paths parameter")
		}
	})

	t.Run("invalid path type", func(t *testing.T) {
		args := map[string]interface{}{
			"paths": []interface{}{123, "file1.txt"},
		}

		tool, _ := registry.Get("multi_file_read")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "ERROR: path must be a string") {
			t.Errorf("Expected error message for invalid path type")
		}
	})
}
