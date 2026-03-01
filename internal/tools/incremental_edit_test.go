package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIncrementalEdit_Replace(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterIncrementalEdit(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	originalContent := `package main

func main() {
	println("old")
}
`
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("incremental_edit")
	if !ok {
		t.Fatal("incremental_edit tool not registered")
	}

	// Replace line 4
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":        "test.go",
		"start_line":  float64(4),
		"end_line":    float64(4),
		"new_content": "\tprintln(\"new\")",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Replaced lines 4-4") {
		t.Errorf("Result should indicate replacement: %s", result)
	}

	// Verify file content
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), "println(\"new\")") {
		t.Errorf("File should contain new content: %s", string(newContent))
	}

	if strings.Contains(string(newContent), "println(\"old\")") {
		t.Errorf("File should not contain old content: %s", string(newContent))
	}
}

func TestIncrementalEdit_Insert(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterIncrementalEdit(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	originalContent := `package main

func main() {
}
`
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("incremental_edit")
	if !ok {
		t.Fatal("incremental_edit tool not registered")
	}

	// Insert at line 4 (after "func main() {")
	// When end_line < start_line, it inserts at start_line position
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":        "test.go",
		"start_line":  float64(4),
		"end_line":    float64(3), // end < start means insert at start position
		"new_content": "\tprintln(\"inserted\")",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Inserted") || !strings.Contains(result, "added: 1") {
		t.Errorf("Result should indicate insertion: %s", result)
	}

	// Verify file content
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), "println(\"inserted\")") {
		t.Errorf("File should contain inserted content: %s", string(newContent))
	}
}

func TestIncrementalEdit_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterIncrementalEdit(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	originalContent := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("incremental_edit")
	if !ok {
		t.Fatal("incremental_edit tool not registered")
	}

	// Delete line 3 (import statement)
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":        "test.go",
		"start_line":  float64(3),
		"end_line":    float64(3),
		"new_content": "",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Deleted lines 3-3") {
		t.Errorf("Result should indicate deletion: %s", result)
	}

	// Verify file content
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if strings.Contains(string(newContent), "import \"fmt\"") {
		t.Errorf("File should not contain deleted import: %s", string(newContent))
	}
}

func TestIncrementalEdit_MultipleLines(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterIncrementalEdit(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	originalContent := `package main

func old1() {
}

func old2() {
}
`
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("incremental_edit")
	if !ok {
		t.Fatal("incremental_edit tool not registered")
	}

	// Replace lines 3-7 with new function
	newFunc := `func newFunc() {
	println("new")
}`
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":        "test.go",
		"start_line":  float64(3),
		"end_line":    float64(7),
		"new_content": newFunc,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Replaced lines 3-7") {
		t.Errorf("Result should indicate multi-line replacement: %s", result)
	}

	// Verify file content
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), "func newFunc()") {
		t.Errorf("File should contain new function: %s", string(newContent))
	}

	if strings.Contains(string(newContent), "func old1()") {
		t.Errorf("File should not contain old1: %s", string(newContent))
	}

	if strings.Contains(string(newContent), "func old2()") {
		t.Errorf("File should not contain old2: %s", string(newContent))
	}
}

func TestIncrementalEdit_AppendToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterIncrementalEdit(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	originalContent := `package main

func main() {
}
`
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("incremental_edit")
	if !ok {
		t.Fatal("incremental_edit tool not registered")
	}

	// Append at end (line 5 is beyond current file)
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":        "test.go",
		"start_line":  float64(5),
		"end_line":    float64(5),
		"new_content": "\nfunc newFunc() {}",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Inserted") {
		t.Errorf("Result should indicate insertion: %s", result)
	}

	// Verify file content
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), "func newFunc()") {
		t.Errorf("File should contain appended function: %s", string(newContent))
	}
}

func TestIncrementalEdit_PreserveNewline(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterIncrementalEdit(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")

	// Test with file ending in newline
	originalContent := "line1\nline2\nline3\n"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("incremental_edit")
	if !ok {
		t.Fatal("incremental_edit tool not registered")
	}

	// Replace line 2
	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":        "test.txt",
		"start_line":  float64(2),
		"end_line":    float64(2),
		"new_content": "modified",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	// Verify file still ends with newline
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.HasSuffix(string(newContent), "\n") {
		t.Error("File should still end with newline")
	}

	expected := "line1\nmodified\nline3\n"
	if string(newContent) != expected {
		t.Errorf("Content mismatch.\nExpected: %q\nGot: %q", expected, string(newContent))
	}
}

func TestIncrementalEdit_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterIncrementalEdit(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	originalContent := `package main

func main() {
}
`
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
			errMsg:  "path must be a string",
		},
		{
			name: "missing start_line",
			args: map[string]interface{}{
				"path": "test.go",
			},
			wantErr: true,
			errMsg:  "start_line must be an integer",
		},
		{
			name: "invalid start_line",
			args: map[string]interface{}{
				"path":        "test.go",
				"start_line":  float64(0),
				"end_line":    float64(1),
				"new_content": "test",
			},
			wantErr: true,
			errMsg:  "start_line must be >= 1",
		},
		{
			name: "invalid end_line",
			args: map[string]interface{}{
				"path":        "test.go",
				"start_line":  float64(5),
				"end_line":    float64(0),
				"new_content": "test",
			},
			wantErr: true,
			errMsg:  "end_line must be >= 1",
		},
		{
			name: "file not found",
			args: map[string]interface{}{
				"path":        "nonexistent.go",
				"start_line":  float64(1),
				"end_line":    float64(1),
				"new_content": "test",
			},
			wantErr: true,
			errMsg:  "failed to read file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := registry.Get("incremental_edit")
			if !ok {
				t.Fatal("incremental_edit tool not registered")
			}

			_, err := tool.Handler(context.Background(), tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Error message = %v, want to contain %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestIncrementalEdit_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterIncrementalEdit(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("incremental_edit")
	if !ok {
		t.Fatal("incremental_edit tool not registered")
	}

	// Insert into empty file
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":        "empty.txt",
		"start_line":  float64(1),
		"end_line":    float64(1),
		"new_content": "first line",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Inserted") {
		t.Errorf("Result should indicate insertion: %s", result)
	}

	// Verify file content
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(newContent) != "first line" {
		t.Errorf("Expected 'first line', got: %q", string(newContent))
	}
}

func TestIncrementalEdit_NoTrailingNewline(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterIncrementalEdit(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")

	// File without trailing newline
	originalContent := "line1\nline2\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("incremental_edit")
	if !ok {
		t.Fatal("incremental_edit tool not registered")
	}

	// Replace line 2
	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":        "test.txt",
		"start_line":  float64(2),
		"end_line":    float64(2),
		"new_content": "modified",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	// Verify file still doesn't end with newline
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if strings.HasSuffix(string(newContent), "\n") {
		t.Error("File should not end with newline (preserving original format)")
	}

	expected := "line1\nmodified\nline3"
	if string(newContent) != expected {
		t.Errorf("Content mismatch.\nExpected: %q\nGot: %q", expected, string(newContent))
	}
}
