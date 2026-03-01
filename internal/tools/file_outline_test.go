package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileOutline_GoFile(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileOutline(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "user.go")
	goContent := `package main

type User struct {
	Name string
	Age  int
}

func NewUser(name string) *User {
	return &User{Name: name}
}

func (u *User) GetName() string {
	return u.Name
}

func (u *User) SetName(name string) {
	u.Name = name
}

func privateHelper() {
	println("private")
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("file_outline")
	if !ok {
		t.Fatal("file_outline tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": "user.go",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	// Check structure
	if !strings.Contains(result, "Language: go") {
		t.Error("Should indicate language")
	}

	if !strings.Contains(result, "User") {
		t.Error("Should include User struct")
	}

	if !strings.Contains(result, "NewUser") {
		t.Error("Should include NewUser function")
	}

	if !strings.Contains(result, "GetName") {
		t.Error("Should include GetName method")
	}

	if !strings.Contains(result, "SetName") {
		t.Error("Should include SetName method")
	}

	// Methods should be indented under User
	lines := strings.Split(result, "\n")
	var foundUser, foundGetName, foundSetName bool
	var userIndent, getNameIndent, setNameIndent int

	for _, line := range lines {
		if strings.Contains(line, "+ User") && strings.Contains(line, "line 3") {
			foundUser = true
			userIndent = len(line) - len(strings.TrimLeft(line, " "))
		}
		if strings.Contains(line, "+ GetName") {
			foundGetName = true
			getNameIndent = len(line) - len(strings.TrimLeft(line, " "))
		}
		if strings.Contains(line, "+ SetName") {
			foundSetName = true
			setNameIndent = len(line) - len(strings.TrimLeft(line, " "))
		}
	}

	if !foundUser || !foundGetName || !foundSetName {
		t.Error("Should find User, GetName, and SetName")
	}

	if getNameIndent <= userIndent || setNameIndent <= userIndent {
		t.Error("Methods should be indented more than their parent struct")
	}
}

func TestFileOutline_IncludePrivate(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileOutline(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

func PublicFunc() {}
func privateFunc() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("file_outline")
	if !ok {
		t.Fatal("file_outline tool not registered")
	}

	// Without private
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":            "test.go",
		"include_private": false,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "PublicFunc") {
		t.Error("Should include PublicFunc")
	}

	if strings.Contains(result, "privateFunc") {
		t.Error("Should not include privateFunc when include_private=false")
	}

	// With private
	result, err = tool.Handler(context.Background(), map[string]interface{}{
		"path":            "test.go",
		"include_private": true,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "PublicFunc") {
		t.Error("Should include PublicFunc")
	}

	if !strings.Contains(result, "privateFunc") {
		t.Error("Should include privateFunc when include_private=true")
	}
}

func TestFileOutline_MaxDepth(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileOutline(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

type User struct {
	Name string
}

func (u *User) GetName() string {
	return u.Name
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("file_outline")
	if !ok {
		t.Fatal("file_outline tool not registered")
	}

	// Max depth 0 - only top level
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":      "test.go",
		"max_depth": float64(0),
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "User") {
		t.Error("Should include User at depth 0")
	}

	if strings.Contains(result, "GetName") {
		t.Error("Should not include GetName at depth 0 (it's at depth 1)")
	}

	// Max depth 1 - include methods
	result, err = tool.Handler(context.Background(), map[string]interface{}{
		"path":      "test.go",
		"max_depth": float64(1),
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "User") {
		t.Error("Should include User")
	}

	if !strings.Contains(result, "GetName") {
		t.Error("Should include GetName at depth 1")
	}
}

func TestFileOutline_MultiLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileOutline(registry, tmpDir)

	tests := []struct {
		name         string
		filename     string
		content      string
		wantContains []string
	}{
		{
			name:     "Python",
			filename: "test.py",
			content: `class User:
    def __init__(self, name):
        self.name = name
    
    def get_name(self):
        return self.name

def create_user(name):
    return User(name)
`,
			wantContains: []string{
				"Language: python",
				"User",
				"get_name",
				"create_user",
			},
		},
		{
			name:     "JavaScript",
			filename: "test.js",
			content: `class User {
    constructor(name) {
        this.name = name;
    }
    
    getName() {
        return this.name;
    }
}

function createUser(name) {
    return new User(name);
}
`,
			wantContains: []string{
				"Language: javascript",
				"User",
				"getName",
				"createUser",
			},
		},
		{
			name:     "Rust",
			filename: "test.rs",
			content: `pub struct User {
    pub name: String,
}

impl User {
    pub fn new(name: String) -> Self {
        User { name }
    }
    
    pub fn get_name(&self) -> &str {
        &self.name
    }
}
`,
			wantContains: []string{
				"Language: rust",
				"User",
				"new",
				"get_name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			tool, ok := registry.Get("file_outline")
			if !ok {
				t.Fatal("file_outline tool not registered")
			}

			result, err := tool.Handler(context.Background(), map[string]interface{}{
				"path": tt.filename,
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

func TestFileOutline_LineNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileOutline(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

func First() {}

func Second() {}

func Third() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("file_outline")
	if !ok {
		t.Fatal("file_outline tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": "test.go",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	// Check line numbers
	if !strings.Contains(result, "First") && !strings.Contains(result, "line 3") {
		t.Error("Should include line number for First")
	}

	if !strings.Contains(result, "Second") && !strings.Contains(result, "line 5") {
		t.Error("Should include line number for Second")
	}

	if !strings.Contains(result, "Third") && !strings.Contains(result, "line 7") {
		t.Error("Should include line number for Third")
	}
}

func TestFileOutline_ExportedIndicator(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileOutline(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

func PublicFunc() {}
func privateFunc() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("file_outline")
	if !ok {
		t.Fatal("file_outline tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":            "test.go",
		"include_private": true,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	// Public should have + indicator
	if !strings.Contains(result, "+ PublicFunc") {
		t.Error("Public function should have + indicator")
	}

	// Private should have - indicator
	if !strings.Contains(result, "- privateFunc") {
		t.Error("Private function should have - indicator")
	}
}

func TestFileOutline_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileOutline(registry, tmpDir)

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
			name: "file not found",
			args: map[string]interface{}{
				"path": "nonexistent.go",
			},
			wantErr: true,
			errMsg:  "file not found",
		},
		{
			name: "invalid path type",
			args: map[string]interface{}{
				"path": 123,
			},
			wantErr: true,
			errMsg:  "path must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := registry.Get("file_outline")
			if !ok {
				t.Fatal("file_outline tool not registered")
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

func TestFileOutline_UnsupportedFile(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileOutline(registry, tmpDir)

	txtFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("plain text"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("file_outline")
	if !ok {
		t.Fatal("file_outline tool not registered")
	}

	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": "test.txt",
	})
	if err == nil {
		t.Error("Should error on unsupported file type")
	}

	if !strings.Contains(err.Error(), "unsupported file type") {
		t.Errorf("Error should mention unsupported file type, got: %v", err)
	}
}
