package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeContextV2_FindDefinitions(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCodeContextV2(registry, tmpDir)

	// Create test file with definitions
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

	tool, ok := registry.Get("code_context")
	if !ok {
		t.Fatal("code_context tool not registered")
	}

	// Search for User type
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"symbol": "User",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "DEFINITIONS") {
		t.Error("Should show definitions section")
	}

	if !strings.Contains(result, "type User") {
		t.Error("Should include User type definition")
	}

	if !strings.Contains(result, "Type: struct") {
		t.Error("Should show symbol type")
	}
}

func TestCodeContextV2_FindUsages(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCodeContextV2(registry, tmpDir)

	// Create files with definition and usages
	defFile := filepath.Join(tmpDir, "user.go")
	defContent := `package main

func Helper() string {
	return "helper"
}
`
	if err := os.WriteFile(defFile, []byte(defContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	usageFile := filepath.Join(tmpDir, "main.go")
	usageContent := `package main

func main() {
	result := Helper()
	println(result)
}
`
	if err := os.WriteFile(usageFile, []byte(usageContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("code_context")
	if !ok {
		t.Fatal("code_context tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"symbol": "Helper",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "USAGES") {
		t.Error("Should show usages section")
	}

	if !strings.Contains(result, "main.go") {
		t.Error("Should include usage file")
	}

	if !strings.Contains(result, "Helper()") {
		t.Error("Should show usage line")
	}
}

func TestCodeContextV2_MultiLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCodeContextV2(registry, tmpDir)

	tests := []struct {
		name     string
		filename string
		content  string
		symbol   string
		wantLang string
	}{
		{
			name:     "Python",
			filename: "test.py",
			content: `class User:
    def __init__(self, name):
        self.name = name
`,
			symbol:   "User",
			wantLang: "python",
		},
		{
			name:     "JavaScript",
			filename: "test.js",
			content: `class User {
    constructor(name) {
        this.name = name;
    }
}
`,
			symbol:   "User",
			wantLang: "javascript",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			tool, ok := registry.Get("code_context")
			if !ok {
				t.Fatal("code_context tool not registered")
			}

			result, err := tool.Handler(context.Background(), map[string]interface{}{
				"symbol": tt.symbol,
			})
			if err != nil {
				t.Fatalf("Handler() error = %v", err)
			}

			if !strings.Contains(result, tt.wantLang) {
				t.Errorf("Should indicate language %s, got: %s", tt.wantLang, result)
			}

			if !strings.Contains(result, tt.symbol) {
				t.Error("Should find the symbol")
			}
		})
	}
}

func TestCodeContextV2_ExcludeTests(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCodeContextV2(registry, tmpDir)

	// Create regular file
	mainFile := filepath.Join(tmpDir, "main.go")
	mainContent := `package main
func TestHelper() {}
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test file
	testFile := filepath.Join(tmpDir, "main_test.go")
	testContent := `package main
func TestHelper() {}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("code_context")
	if !ok {
		t.Fatal("code_context tool not registered")
	}

	// With tests excluded
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"symbol":        "TestHelper",
		"include_tests": false,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if strings.Contains(result, "main_test.go") {
		t.Error("Should not include test files when include_tests=false")
	}

	// With tests included
	result, err = tool.Handler(context.Background(), map[string]interface{}{
		"symbol":        "TestHelper",
		"include_tests": true,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "main_test.go") {
		t.Error("Should include test files when include_tests=true")
	}
}

func TestCodeContextV2_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCodeContextV2(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(goFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("code_context")
	if !ok {
		t.Fatal("code_context tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"symbol": "NonExistent",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "not found") {
		t.Error("Should indicate symbol not found")
	}
}

func TestCodeContextV2_ContextLines(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCodeContextV2(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

func main() {
	line1()
	line2()
	Helper()
	line4()
	line5()
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("code_context")
	if !ok {
		t.Fatal("code_context tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"symbol":        "Helper",
		"context_lines": float64(1),
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "line2()") {
		t.Error("Should include 1 line before")
	}

	if !strings.Contains(result, "line4()") {
		t.Error("Should include 1 line after")
	}

	if strings.Contains(result, "line1()") {
		t.Error("Should not include 2 lines before")
	}
}
