package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchAndRead(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "test1.go"), []byte("package main\nfunc main() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test2.go"), []byte("package utils\nfunc Helper() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# README"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "test3.go"), []byte("package sub\nfunc Sub() {}"), 0644)

	registry := NewRegistry()
	RegisterSearchAndRead(registry, tmpDir)

	ctx := context.Background()

	t.Run("search for go files", func(t *testing.T) {
		args := map[string]interface{}{
			"pattern": "*.go",
		}

		tool, _ := registry.Get("search_and_read")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Found 3 files") {
			t.Errorf("Expected to find 3 go files, got: %s", result)
		}

		if !strings.Contains(result, "package main") {
			t.Errorf("Expected test1.go content in result")
		}
	})

	t.Run("search with grep filter", func(t *testing.T) {
		args := map[string]interface{}{
			"pattern":     "*.go",
			"grep_filter": "func main",
		}

		tool, _ := registry.Get("search_and_read")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "filtered to 1 containing") {
			t.Errorf("Expected filter message, got: %s", result)
		}

		if !strings.Contains(result, "package main") {
			t.Errorf("Expected filtered file content")
		}

		if strings.Contains(result, "package utils") {
			t.Errorf("Should not contain filtered out file")
		}
	})

	t.Run("limit max files", func(t *testing.T) {
		args := map[string]interface{}{
			"pattern":   "*.go",
			"max_files": float64(2),
		}

		tool, _ := registry.Get("search_and_read")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "reading 2 files") {
			t.Errorf("Expected to read only 2 files, got: %s", result)
		}
	})

	t.Run("search in subdirectory", func(t *testing.T) {
		args := map[string]interface{}{
			"pattern":   "*.go",
			"directory": "subdir",
		}

		tool, _ := registry.Get("search_and_read")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Found 1 files") {
			t.Errorf("Expected to find 1 file in subdir, got: %s", result)
		}

		if !strings.Contains(result, "package sub") {
			t.Errorf("Expected subdir file content")
		}
	})

	t.Run("no matching files", func(t *testing.T) {
		args := map[string]interface{}{
			"pattern": "*.java",
		}

		tool, _ := registry.Get("search_and_read")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "No files matching") {
			t.Errorf("Expected no files message, got: %s", result)
		}
	})

	t.Run("grep filter with no matches", func(t *testing.T) {
		args := map[string]interface{}{
			"pattern":     "*.go",
			"grep_filter": "nonexistent_function",
		}

		tool, _ := registry.Get("search_and_read")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "but none contain") {
			t.Errorf("Expected filter no match message, got: %s", result)
		}
	})

	t.Run("missing pattern parameter", func(t *testing.T) {
		args := map[string]interface{}{}

		tool, _ := registry.Get("search_and_read")
		_, err := tool.Handler(ctx, args)

		if err == nil {
			t.Error("Expected error for missing pattern")
		}
	})
}
