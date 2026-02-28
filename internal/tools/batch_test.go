package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBatchTools(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a test registry
	registry := NewRegistry()
	RegisterFileTools(registry, tmpDir)
	RegisterBatchTools(registry)

	ctx := context.Background()

	// Test batch_tools with multiple file operations
	testFile1 := filepath.Join(tmpDir, "test1.txt")
	testFile2 := filepath.Join(tmpDir, "test2.txt")

	// Prepare batch call arguments
	batchArgs := map[string]interface{}{
		"calls": []interface{}{
			map[string]interface{}{
				"name": "write_file",
				"arguments": map[string]interface{}{
					"path":    "test1.txt",
					"content": "Hello from test1",
				},
			},
			map[string]interface{}{
				"name": "write_file",
				"arguments": map[string]interface{}{
					"path":    "test2.txt",
					"content": "Hello from test2",
				},
			},
			map[string]interface{}{
				"name": "read_file",
				"arguments": map[string]interface{}{
					"path": "test1.txt",
				},
			},
		},
	}

	// Execute batch_tools
	tool, ok := registry.Get("batch_tools")
	if !ok {
		t.Fatal("batch_tools not found in registry")
	}

	result, err := tool.Handler(ctx, batchArgs)
	if err != nil {
		t.Fatalf("batch_tools execution failed: %v", err)
	}

	// Verify result contains success messages
	if !strings.Contains(result, "SUCCESS") {
		t.Errorf("Expected SUCCESS in result, got: %s", result)
	}

	if !strings.Contains(result, "3/3 successful") {
		t.Errorf("Expected 3/3 successful, got: %s", result)
	}

	// Verify files were created
	if _, err := os.Stat(testFile1); os.IsNotExist(err) {
		t.Error("test1.txt was not created")
	}

	if _, err := os.Stat(testFile2); os.IsNotExist(err) {
		t.Error("test2.txt was not created")
	}

	// Verify file contents
	content1, err := os.ReadFile(testFile1)
	if err != nil {
		t.Fatalf("Failed to read test1.txt: %v", err)
	}
	if string(content1) != "Hello from test1" {
		t.Errorf("Expected 'Hello from test1', got: %s", string(content1))
	}

	content2, err := os.ReadFile(testFile2)
	if err != nil {
		t.Fatalf("Failed to read test2.txt: %v", err)
	}
	if string(content2) != "Hello from test2" {
		t.Errorf("Expected 'Hello from test2', got: %s", string(content2))
	}
}

func TestBatchToolsWithError(t *testing.T) {
	tmpDir := t.TempDir()

	registry := NewRegistry()
	RegisterFileTools(registry, tmpDir)
	RegisterBatchTools(registry)

	ctx := context.Background()

	// Test batch with one failing operation
	batchArgs := map[string]interface{}{
		"calls": []interface{}{
			map[string]interface{}{
				"name": "write_file",
				"arguments": map[string]interface{}{
					"path":    "test.txt",
					"content": "Hello",
				},
			},
			map[string]interface{}{
				"name": "read_file",
				"arguments": map[string]interface{}{
					"path": "nonexistent.txt",
				},
			},
		},
	}

	tool, _ := registry.Get("batch_tools")
	result, err := tool.Handler(ctx, batchArgs)

	// Should not return error, but result should show partial success
	if err != nil {
		t.Fatalf("batch_tools should not return error for partial failures: %v", err)
	}

	if !strings.Contains(result, "1/2 successful") {
		t.Errorf("Expected 1/2 successful, got: %s", result)
	}

	if !strings.Contains(result, "ERROR") {
		t.Errorf("Expected ERROR in result for failed operation, got: %s", result)
	}
}

func TestBatchToolsInvalidTool(t *testing.T) {
	tmpDir := t.TempDir()

	registry := NewRegistry()
	RegisterFileTools(registry, tmpDir)
	RegisterBatchTools(registry)

	ctx := context.Background()

	// Test batch with invalid tool name
	batchArgs := map[string]interface{}{
		"calls": []interface{}{
			map[string]interface{}{
				"name": "invalid_tool",
				"arguments": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}

	tool, _ := registry.Get("batch_tools")
	result, err := tool.Handler(ctx, batchArgs)

	if err != nil {
		t.Fatalf("batch_tools should not return error for invalid tool: %v", err)
	}

	if !strings.Contains(result, "not found") {
		t.Errorf("Expected 'not found' error in result, got: %s", result)
	}
}
