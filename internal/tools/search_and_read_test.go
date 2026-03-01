package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchAndRead_FullMode(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSearchAndRead(registry, tmpDir)

	// Create test files
	file1 := filepath.Join(tmpDir, "test1.go")
	if err := os.WriteFile(file1, []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	file2 := filepath.Join(tmpDir, "test2.go")
	if err := os.WriteFile(file2, []byte("package main\nfunc helper() {}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("search_and_read")
	if !ok {
		t.Fatal("search_and_read tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"pattern": "*.go",
		"mode":    "full",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Found 2 files") {
		t.Error("Should find 2 files")
	}

	if !strings.Contains(result, "package main") {
		t.Error("Should include full file content")
	}
}

func TestSearchAndRead_SummaryMode(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSearchAndRead(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "user.go")
	goContent := `package main

type User struct {
	Name string
}

func NewUser(name string) *User {
	return &User{Name: name}
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("search_and_read")
	if !ok {
		t.Fatal("search_and_read tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"pattern": "*.go",
		"mode":    "summary",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "summary") {
		t.Error("Should indicate summary mode")
	}

	if !strings.Contains(result, "Token savings") {
		t.Error("Should show token savings")
	}

	if !strings.Contains(result, "user.go") {
		t.Error("Should include filename")
	}

	// Should NOT include full code
	if strings.Contains(result, "return &User{Name: name}") {
		t.Error("Summary mode should not include full code")
	}
}

func TestSearchAndRead_OutlineMode(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSearchAndRead(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "user.go")
	goContent := `package main

type User struct {
	Name string
}

func NewUser(name string) *User {
	return &User{Name: name}
}

func (u *User) GetName() string {
	return u.Name
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("search_and_read")
	if !ok {
		t.Fatal("search_and_read tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"pattern": "*.go",
		"mode":    "outline",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "outline") {
		t.Error("Should indicate outline mode")
	}

	if !strings.Contains(result, "Types:") {
		t.Error("Should show types section")
	}

	if !strings.Contains(result, "User") {
		t.Error("Should include User type")
	}

	if !strings.Contains(result, "Functions:") {
		t.Error("Should show functions section")
	}

	if !strings.Contains(result, "NewUser") {
		t.Error("Should include NewUser function")
	}

	// Should NOT include full code
	if strings.Contains(result, "return u.Name") {
		t.Error("Outline mode should not include full code")
	}
}

func TestSearchAndRead_GrepFilter(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSearchAndRead(registry, tmpDir)

	file1 := filepath.Join(tmpDir, "test1.go")
	if err := os.WriteFile(file1, []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	file2 := filepath.Join(tmpDir, "test2.go")
	if err := os.WriteFile(file2, []byte("package main\nfunc helper() {}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("search_and_read")
	if !ok {
		t.Fatal("search_and_read tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"pattern":     "*.go",
		"grep_filter": "helper",
		"mode":        "full",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "filtered to 1") {
		t.Error("Should filter to 1 file")
	}

	if !strings.Contains(result, "test2.go") {
		t.Error("Should include file with helper")
	}

	if strings.Contains(result, "test1.go") {
		t.Error("Should not include file without helper")
	}
}

func TestSearchAndRead_MaxFiles(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSearchAndRead(registry, tmpDir)

	// Create 5 files
	for i := 1; i <= 5; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("test%d.go", i))
		if err := os.WriteFile(filename, []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	tool, ok := registry.Get("search_and_read")
	if !ok {
		t.Fatal("search_and_read tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"pattern":   "*.go",
		"max_files": float64(3),
		"mode":      "summary",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Found 5 files") {
		t.Error("Should report 5 files found")
	}

	if !strings.Contains(result, "reading 3 files") {
		t.Error("Should limit to 3 files")
	}
}

func TestSearchAndRead_MultiLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSearchAndRead(registry, tmpDir)

	// Create files in different languages
	goFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(goFile, []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	pyFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(pyFile, []byte("def main():\n    pass"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	jsFile := filepath.Join(tmpDir, "test.js")
	if err := os.WriteFile(jsFile, []byte("function main() {}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("search_and_read")
	if !ok {
		t.Fatal("search_and_read tool not registered")
	}

	// Test Python files
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"pattern": "*.py",
		"mode":    "summary",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "python") {
		t.Error("Should detect Python language")
	}

	// Test JavaScript files
	result, err = tool.Handler(context.Background(), map[string]interface{}{
		"pattern": "*.js",
		"mode":    "summary",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "javascript") {
		t.Error("Should detect JavaScript language")
	}
}

func TestSearchAndRead_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSearchAndRead(registry, tmpDir)

	txtFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("plain text"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("search_and_read")
	if !ok {
		t.Fatal("search_and_read tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"pattern": "*.txt",
		"mode":    "summary",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "unsupported format") {
		t.Error("Should indicate unsupported format")
	}
}

func TestSearchAndRead_NoMatches(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSearchAndRead(registry, tmpDir)

	tool, ok := registry.Get("search_and_read")
	if !ok {
		t.Fatal("search_and_read tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"pattern": "*.nonexistent",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "No files matching") {
		t.Error("Should indicate no files found")
	}
}

func TestSearchAndRead_IncludePrivate(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSearchAndRead(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

func PublicFunc() {}
func privateFunc() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("search_and_read")
	if !ok {
		t.Fatal("search_and_read tool not registered")
	}

	// Without private
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"pattern":         "*.go",
		"mode":            "outline",
		"include_private": false,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if strings.Contains(result, "privateFunc") {
		t.Error("Should not include private functions when include_private=false")
	}

	// With private
	result, err = tool.Handler(context.Background(), map[string]interface{}{
		"pattern":         "*.go",
		"mode":            "outline",
		"include_private": true,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "privateFunc") {
		t.Error("Should include private functions when include_private=true")
	}
}
