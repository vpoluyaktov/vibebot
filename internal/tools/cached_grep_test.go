package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCachedGrep_BasicSearch(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCachedGrep(registry, tmpDir)

	// Create test files
	file1 := filepath.Join(tmpDir, "test1.go")
	content1 := `package main

func Hello() {
	println("hello world")
}
`
	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	file2 := filepath.Join(tmpDir, "test2.go")
	content2 := `package main

func Goodbye() {
	println("goodbye world")
}
`
	if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("cached_grep")
	if !ok {
		t.Fatal("cached_grep tool not registered")
	}

	// Search for "world"
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"query": "world",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Found 2 matches") {
		t.Errorf("Should find 2 matches, got: %s", result)
	}

	if !strings.Contains(result, "test1.go") {
		t.Error("Should include test1.go")
	}

	if !strings.Contains(result, "test2.go") {
		t.Error("Should include test2.go")
	}

	if !strings.Contains(result, "hello world") {
		t.Error("Should include matched line from test1.go")
	}

	if !strings.Contains(result, "goodbye world") {
		t.Error("Should include matched line from test2.go")
	}
}

func TestCachedGrep_FilePattern(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCachedGrep(registry, tmpDir)

	// Create Go file
	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main
func test() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create Python file
	pyFile := filepath.Join(tmpDir, "test.py")
	pyContent := `def test():
    pass
`
	if err := os.WriteFile(pyFile, []byte(pyContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("cached_grep")
	if !ok {
		t.Fatal("cached_grep tool not registered")
	}

	// Search only in Go files
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"query":        "test",
		"file_pattern": "*.go",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "test.go") {
		t.Error("Should include test.go")
	}

	if strings.Contains(result, "test.py") {
		t.Error("Should not include test.py when pattern is *.go")
	}
}

func TestCachedGrep_ContextLines(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCachedGrep(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	content := `line 1
line 2
line 3 match
line 4
line 5
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("cached_grep")
	if !ok {
		t.Fatal("cached_grep tool not registered")
	}

	// Search with 1 context line
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"query":         "match",
		"context_lines": float64(1),
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	// Should show line 2, 3 (match), and 4
	if !strings.Contains(result, "line 2") {
		t.Error("Should include line 2 (1 line before match)")
	}

	if !strings.Contains(result, "line 3 match") {
		t.Error("Should include matched line")
	}

	if !strings.Contains(result, "line 4") {
		t.Error("Should include line 4 (1 line after match)")
	}

	if strings.Contains(result, "line 1") {
		t.Error("Should not include line 1 (2 lines before match)")
	}

	if strings.Contains(result, "line 5") {
		t.Error("Should not include line 5 (2 lines after match)")
	}
}

func TestCachedGrep_CaseSensitive(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCachedGrep(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	content := `Hello World
hello world
HELLO WORLD
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("cached_grep")
	if !ok {
		t.Fatal("cached_grep tool not registered")
	}

	// Case-insensitive search (default)
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"query": "hello",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Found 3 matches") {
		t.Errorf("Case-insensitive should find 3 matches, got: %s", result)
	}

	// Case-sensitive search
	result, err = tool.Handler(context.Background(), map[string]interface{}{
		"query":          "hello",
		"case_sensitive": true,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Found 1 match") {
		t.Errorf("Case-sensitive should find 1 match, got: %s", result)
	}
}

func TestCachedGrep_MaxResults(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCachedGrep(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	var content strings.Builder
	for i := 1; i <= 10; i++ {
		content.WriteString(fmt.Sprintf("line %d with match\n", i))
	}
	if err := os.WriteFile(testFile, []byte(content.String()), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("cached_grep")
	if !ok {
		t.Fatal("cached_grep tool not registered")
	}

	// Limit to 3 results
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"query":       "match",
		"max_results": float64(3),
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Found 10 matches") {
		t.Error("Should report total of 10 matches")
	}

	if !strings.Contains(result, "and 7 more matches") {
		t.Error("Should indicate there are more matches")
	}

	// Count how many results are shown
	resultCount := strings.Count(result, "test.go:")
	if resultCount != 3 {
		t.Errorf("Should show 3 results, got %d", resultCount)
	}
}

func TestCachedGrep_Caching(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCachedGrep(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main
func test() {}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("cached_grep")
	if !ok {
		t.Fatal("cached_grep tool not registered")
	}

	// First search
	result1, err := tool.Handler(context.Background(), map[string]interface{}{
		"query": "test",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if strings.Contains(result1, "from cache") {
		t.Error("First search should not be from cache")
	}

	// Second search (should be cached)
	result2, err := tool.Handler(context.Background(), map[string]interface{}{
		"query": "test",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result2, "from cache") {
		t.Error("Second search should be from cache")
	}

	// Results should be the same
	if result1 != strings.Replace(result2, " (from cache)", "", 1) {
		t.Error("Cached results should match original results")
	}
}

func TestCachedGrep_CacheExpiry(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCachedGrep(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main
func test() {}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("cached_grep")
	if !ok {
		t.Fatal("cached_grep tool not registered")
	}

	// First search
	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"query": "test_expiry",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	// Manually expire the cache by setting old timestamp
	cacheKey := generateCacheKey(tmpDir, "test_expiry", "", false)
	globalSearchCache.mu.Lock()
	if entry, exists := globalSearchCache.entries[cacheKey]; exists {
		entry.Timestamp = time.Now().Add(-10 * time.Minute) // Expire it
	}
	globalSearchCache.mu.Unlock()

	// Search again (should not be from cache)
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"query": "test_expiry",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if strings.Contains(result, "from cache") {
		t.Error("Expired cache should not be used")
	}
}

func TestCachedGrep_NoMatches(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCachedGrep(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main
func test() {}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("cached_grep")
	if !ok {
		t.Fatal("cached_grep tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"query": "nonexistent",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Found 0 matches") {
		t.Error("Should report 0 matches")
	}

	if !strings.Contains(result, "No matches found") {
		t.Error("Should indicate no matches found")
	}
}

func TestCachedGrep_LineNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCachedGrep(registry, tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")
	content := `line 1
line 2
line 3 match
line 4
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("cached_grep")
	if !ok {
		t.Fatal("cached_grep tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"query": "match",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	// Should show line number
	if !strings.Contains(result, "test.go:3") {
		t.Error("Should include file path with line number")
	}

	// Should show line-numbered context
	if !strings.Contains(result, "   3 |") {
		t.Error("Should include line numbers in context")
	}
}

func TestCachedGrep_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterCachedGrep(registry, tmpDir)

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing query",
			args:    map[string]interface{}{},
			wantErr: true,
			errMsg:  "query must be a string",
		},
		{
			name: "invalid query type",
			args: map[string]interface{}{
				"query": 123,
			},
			wantErr: true,
			errMsg:  "query must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := registry.Get("cached_grep")
			if !ok {
				t.Fatal("cached_grep tool not registered")
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
