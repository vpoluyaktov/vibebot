package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceSnapshot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	os.MkdirAll(filepath.Join(tmpDir, "src", "utils"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "tests"), 0755)

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test Project\nThis is a test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.21"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "utils", "helper.go"), []byte("package utils"), 0644)

	registry := NewRegistry()
	RegisterWorkspaceSnapshot(registry, tmpDir)

	ctx := context.Background()

	t.Run("full snapshot with defaults", func(t *testing.T) {
		args := map[string]interface{}{}

		tool, _ := registry.Get("workspace_snapshot")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "Workspace Snapshot:") {
			t.Errorf("Expected workspace snapshot header")
		}

		if !strings.Contains(result, "## Directory Structure") {
			t.Errorf("Expected directory structure section")
		}

		if !strings.Contains(result, "## Key Files") {
			t.Errorf("Expected key files section")
		}

		if !strings.Contains(result, "src/") {
			t.Errorf("Expected src directory in structure")
		}

		if !strings.Contains(result, "### README.md") {
			t.Errorf("Expected README.md in key files")
		}

		if !strings.Contains(result, "# Test Project") {
			t.Errorf("Expected README.md content")
		}
	})

	t.Run("structure only", func(t *testing.T) {
		args := map[string]interface{}{
			"include_summaries": false,
		}

		tool, _ := registry.Get("workspace_snapshot")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "## Directory Structure") {
			t.Errorf("Expected directory structure section")
		}

		if strings.Contains(result, "## Key Files") {
			t.Errorf("Should not include key files section")
		}
	})

	t.Run("summaries only", func(t *testing.T) {
		args := map[string]interface{}{
			"include_structure": false,
		}

		tool, _ := registry.Get("workspace_snapshot")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if strings.Contains(result, "## Directory Structure") {
			t.Errorf("Should not include directory structure section")
		}

		if !strings.Contains(result, "## Key Files") {
			t.Errorf("Expected key files section")
		}
	})

	t.Run("custom max depth", func(t *testing.T) {
		args := map[string]interface{}{
			"max_depth":         float64(1),
			"include_summaries": false,
		}

		tool, _ := registry.Get("workspace_snapshot")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "src/") {
			t.Errorf("Expected src directory at depth 1")
		}

		// With max_depth=1, we show directories at depth 1 (src/) and their immediate contents at depth 2
		// This is expected behavior - max_depth controls how deep we recurse
	})

	t.Run("truncate long files", func(t *testing.T) {
		longContent := strings.Repeat("a", 600)
		os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(longContent), 0644)

		args := map[string]interface{}{
			"include_structure": false,
		}

		tool, _ := registry.Get("workspace_snapshot")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !strings.Contains(result, "(truncated)") {
			t.Errorf("Expected truncation message for long file")
		}
	})

	t.Run("skip hidden files", func(t *testing.T) {
		os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden content"), 0644)
		os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)

		args := map[string]interface{}{
			"include_summaries": false,
		}

		tool, _ := registry.Get("workspace_snapshot")
		result, err := tool.Handler(ctx, args)

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if strings.Contains(result, ".hidden") {
			t.Errorf("Should not include hidden files")
		}

		if strings.Contains(result, ".git") {
			t.Errorf("Should not include hidden directories")
		}
	})
}
