package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeContext(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc ProcessMessage() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "utils.go"), []byte("package utils\nfunc ProcessMessage() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("package main\nfunc TestProcessMessage() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# ProcessMessage function"), 0644)

	registry := NewRegistry()
	RegisterCodeContext(registry, tmpDir)

	ctx := context.Background()

	t.Run("find symbol in go files", func(t *testing.T) {
		args := map[string]interface{}{
			"symbol": "ProcessMessage",
		}

		tool, _ := registry.Get("code_context")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Found symbol 'ProcessMessage' in 3 go files") {
			t.Errorf("Expected to find symbol in 3 files, got: %s", result)
		}

		if !strings.Contains(result, "package main") {
			t.Errorf("Expected main.go content")
		}

		if !strings.Contains(result, "package utils") {
			t.Errorf("Expected utils.go content")
		}
	})

	t.Run("exclude test files", func(t *testing.T) {
		args := map[string]interface{}{
			"symbol":        "ProcessMessage",
			"include_tests": false,
		}

		tool, _ := registry.Get("code_context")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Found symbol 'ProcessMessage' in 2 go files") {
			t.Errorf("Expected to find symbol in 2 files (excluding tests), got: %s", result)
		}

		if strings.Contains(result, "TestProcessMessage") {
			t.Errorf("Should not include test file content")
		}
	})

	t.Run("symbol not found", func(t *testing.T) {
		args := map[string]interface{}{
			"symbol": "NonexistentFunction",
		}

		tool, _ := registry.Get("code_context")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "not found") {
			t.Errorf("Expected not found message, got: %s", result)
		}
	})

	t.Run("different language", func(t *testing.T) {
		// Create Python file
		os.WriteFile(filepath.Join(tmpDir, "test.py"), []byte("def process_data():\n    pass"), 0644)

		args := map[string]interface{}{
			"symbol":   "process_data",
			"language": "python",
		}

		tool, _ := registry.Get("code_context")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "python files") {
			t.Errorf("Expected python files message, got: %s", result)
		}

		if !strings.Contains(result, "def process_data") {
			t.Errorf("Expected Python file content")
		}
	})

	t.Run("limit to 5 files", func(t *testing.T) {
		// Create 6 files with the same symbol
		for i := 1; i <= 6; i++ {
			os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i)), []byte("package test\nfunc CommonFunc() {}"), 0644)
		}

		args := map[string]interface{}{
			"symbol": "CommonFunc",
		}

		tool, _ := registry.Get("code_context")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Showing first 5 of 6 files") {
			t.Errorf("Expected limit message, got: %s", result)
		}
	})

	t.Run("missing symbol parameter", func(t *testing.T) {
		args := map[string]interface{}{}

		tool, _ := registry.Get("code_context")
		_, err := tool.Handler(ctx, args)

		if err == nil {
			t.Error("Expected error for missing symbol")
		}
	})
}
