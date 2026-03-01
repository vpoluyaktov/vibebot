package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSymbolDefinition_FindFunction(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSymbolDefinition(registry, tmpDir)

	// Create test files
	goFile := filepath.Join(tmpDir, "user.go")
	goContent := `package main

import "fmt"

type User struct {
	Name string
}

func NewUser(name string) *User {
	return &User{Name: name}
}

func (u *User) GetName() string {
	return u.Name
}

func privateHelper() {
	fmt.Println("private")
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("symbol_definition")
	if !ok {
		t.Fatal("symbol_definition tool not registered")
	}

	tests := []struct {
		name         string
		symbol       string
		wantContains []string
		wantCount    int
	}{
		{
			name:   "find function",
			symbol: "NewUser",
			wantContains: []string{
				"Found 1 definition",
				"user.go",
				"Type: function",
				"func NewUser",
				"return &User{Name: name}",
			},
			wantCount: 1,
		},
		{
			name:   "find method",
			symbol: "GetName",
			wantContains: []string{
				"Found 1 definition",
				"Type: method",
				"GetName",
				"return u.Name",
			},
			wantCount: 1,
		},
		{
			name:   "find type",
			symbol: "User",
			wantContains: []string{
				"Found 1 definition",
				"Type: struct",
				"type User",
			},
			wantCount: 1,
		},
		{
			name:   "find private function",
			symbol: "privateHelper",
			wantContains: []string{
				"Found 1 definition",
				"privateHelper",
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Handler(context.Background(), map[string]interface{}{
				"symbol": tt.symbol,
			})
			if err != nil {
				t.Fatalf("Handler() error = %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("Result missing expected content: %q\nGot:\n%s", want, result)
				}
			}
		})
	}
}

func TestSymbolDefinition_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSymbolDefinition(registry, tmpDir)

	// Create multiple files with same symbol name
	file1 := filepath.Join(tmpDir, "user.go")
	content1 := `package main

func Process() {
	println("file1")
}
`
	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	file2 := filepath.Join(tmpDir, "admin.go")
	content2 := `package main

func Process() {
	println("file2")
}
`
	if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("symbol_definition")
	if !ok {
		t.Fatal("symbol_definition tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"symbol": "Process",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Found 2 definition") {
		t.Errorf("Should find 2 definitions, got: %s", result)
	}

	if !strings.Contains(result, "user.go") {
		t.Error("Should include user.go")
	}

	if !strings.Contains(result, "admin.go") {
		t.Error("Should include admin.go")
	}
}

func TestSymbolDefinition_FilePattern(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSymbolDefinition(registry, tmpDir)

	// Create Go file
	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

func Helper() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create Python file with same symbol
	pyFile := filepath.Join(tmpDir, "test.py")
	pyContent := `def Helper():
    pass
`
	if err := os.WriteFile(pyFile, []byte(pyContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("symbol_definition")
	if !ok {
		t.Fatal("symbol_definition tool not registered")
	}

	// Search only Go files
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"symbol":       "Helper",
		"file_pattern": "*.go",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Found 1 definition") {
		t.Errorf("Should find 1 definition with pattern *.go, got: %s", result)
	}

	if !strings.Contains(result, "test.go") {
		t.Error("Should include test.go")
	}

	if strings.Contains(result, "test.py") {
		t.Error("Should not include test.py when pattern is *.go")
	}
}

func TestSymbolDefinition_IncludeBody(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSymbolDefinition(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

func Calculate(x, y int) int {
	result := x + y
	return result
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("symbol_definition")
	if !ok {
		t.Fatal("symbol_definition tool not registered")
	}

	// Test with body
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"symbol":       "Calculate",
		"include_body": true,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "result := x + y") {
		t.Error("Should include function body when include_body=true")
	}

	// Test without body
	result, err = tool.Handler(context.Background(), map[string]interface{}{
		"symbol":       "Calculate",
		"include_body": false,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if strings.Contains(result, "result := x + y") {
		t.Error("Should not include function body when include_body=false")
	}

	if !strings.Contains(result, "Signature:") {
		t.Error("Should still include signature when include_body=false")
	}
}

func TestSymbolDefinition_MultiLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSymbolDefinition(registry, tmpDir)

	tests := []struct {
		name     string
		filename string
		content  string
		symbol   string
		wantType string
	}{
		{
			name:     "Python class",
			filename: "user.py",
			content: `class User:
    def __init__(self, name):
        self.name = name
`,
			symbol:   "User",
			wantType: "class",
		},
		{
			name:     "JavaScript function",
			filename: "utils.js",
			content: `function createUser(name) {
    return { name };
}
`,
			symbol:   "createUser",
			wantType: "function",
		},
		{
			name:     "Java class",
			filename: "User.java",
			content: `public class User {
    private String name;
    
    public User(String name) {
        this.name = name;
    }
}
`,
			symbol:   "User",
			wantType: "class",
		},
		{
			name:     "Rust struct",
			filename: "user.rs",
			content: `pub struct User {
    pub name: String,
}
`,
			symbol:   "User",
			wantType: "struct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated subdirectory for this test
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// Create a new registry with isolated workspace
			testRegistry := NewRegistry()
			RegisterSymbolDefinition(testRegistry, testDir)

			filePath := filepath.Join(testDir, tt.filename)
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			tool, ok := testRegistry.Get("symbol_definition")
			if !ok {
				t.Fatal("symbol_definition tool not registered")
			}

			result, err := tool.Handler(context.Background(), map[string]interface{}{
				"symbol": tt.symbol,
			})
			if err != nil {
				t.Fatalf("Handler() error = %v", err)
			}

			if !strings.Contains(result, "Found 1 definition") {
				t.Errorf("Should find definition, got: %s", result)
			}

			if !strings.Contains(result, fmt.Sprintf("Type: %s", tt.wantType)) {
				t.Errorf("Should have type %s, got: %s", tt.wantType, result)
			}

			if !strings.Contains(result, tt.filename) {
				t.Errorf("Should include filename %s, got: %s", tt.filename, result)
			}
		})
	}
}

func TestSymbolDefinition_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSymbolDefinition(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

func Exists() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("symbol_definition")
	if !ok {
		t.Fatal("symbol_definition tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"symbol": "DoesNotExist",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "not found") {
		t.Errorf("Should indicate symbol not found, got: %s", result)
	}
}

func TestSymbolDefinition_LineNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSymbolDefinition(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

import "fmt"

func FirstFunc() {
	fmt.Println("first")
}

func SecondFunc() {
	fmt.Println("second")
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("symbol_definition")
	if !ok {
		t.Fatal("symbol_definition tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"symbol": "SecondFunc",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	// Should show line range
	if !strings.Contains(result, "test.go:9-") {
		t.Errorf("Should include line numbers, got: %s", result)
	}

	// Should show line numbers in definition
	if !strings.Contains(result, "   9 |") {
		t.Error("Should include line-numbered code")
	}
}

func TestSymbolDefinition_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSymbolDefinition(registry, tmpDir)

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing symbol",
			args:    map[string]interface{}{},
			wantErr: true,
			errMsg:  "symbol must be a string",
		},
		{
			name: "invalid symbol type",
			args: map[string]interface{}{
				"symbol": 123,
			},
			wantErr: true,
			errMsg:  "symbol must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := registry.Get("symbol_definition")
			if !ok {
				t.Fatal("symbol_definition tool not registered")
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
