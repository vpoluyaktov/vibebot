package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSmartEdit_AddImportGo(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSmartEdit(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

func main() {
	println("hello")
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("smart_edit")
	if !ok {
		t.Fatal("smart_edit tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":      "test.go",
		"operation": "add_import",
		"value":     "fmt",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Successfully performed add_import") {
		t.Errorf("Should indicate success, got: %s", result)
	}

	// Verify file was modified
	newContent, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), `import "fmt"`) {
		t.Errorf("Should add import statement, got: %s", string(newContent))
	}
}

func TestSmartEdit_AddImportPython(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSmartEdit(registry, tmpDir)

	pyFile := filepath.Join(tmpDir, "test.py")
	pyContent := `import os

def main():
    print("hello")
`
	if err := os.WriteFile(pyFile, []byte(pyContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("smart_edit")
	if !ok {
		t.Fatal("smart_edit tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":      "test.py",
		"operation": "add_import",
		"value":     "import sys",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "python") {
		t.Error("Should detect Python language")
	}

	newContent, err := os.ReadFile(pyFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), "import sys") {
		t.Errorf("Should add import statement, got: %s", string(newContent))
	}
}

func TestSmartEdit_AddImportJavaScript(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSmartEdit(registry, tmpDir)

	jsFile := filepath.Join(tmpDir, "test.js")
	jsContent := `import React from 'react';

function App() {
    return <div>Hello</div>;
}
`
	if err := os.WriteFile(jsFile, []byte(jsContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("smart_edit")
	if !ok {
		t.Fatal("smart_edit tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":      "test.js",
		"operation": "add_import",
		"value":     "import { useState } from 'react';",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "javascript") {
		t.Error("Should detect JavaScript language")
	}

	newContent, err := os.ReadFile(jsFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), "useState") {
		t.Errorf("Should add import statement, got: %s", string(newContent))
	}
}

func TestSmartEdit_AddImportJava(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSmartEdit(registry, tmpDir)

	javaFile := filepath.Join(tmpDir, "Test.java")
	javaContent := `package com.example;

import java.util.List;

public class Test {
}
`
	if err := os.WriteFile(javaFile, []byte(javaContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("smart_edit")
	if !ok {
		t.Fatal("smart_edit tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":      "Test.java",
		"operation": "add_import",
		"value":     "import java.util.Map",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "java") {
		t.Error("Should detect Java language")
	}

	newContent, err := os.ReadFile(javaFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), "import java.util.Map;") {
		t.Errorf("Should add import with semicolon, got: %s", string(newContent))
	}
}

func TestSmartEdit_AddImportRust(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSmartEdit(registry, tmpDir)

	rsFile := filepath.Join(tmpDir, "test.rs")
	rsContent := `use std::collections::HashMap;

fn main() {
    println!("hello");
}
`
	if err := os.WriteFile(rsFile, []byte(rsContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("smart_edit")
	if !ok {
		t.Fatal("smart_edit tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":      "test.rs",
		"operation": "add_import",
		"value":     "use std::io;",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "rust") {
		t.Error("Should detect Rust language")
	}

	newContent, err := os.ReadFile(rsFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), "use std::io;") {
		t.Errorf("Should add use statement, got: %s", string(newContent))
	}
}

func TestSmartEdit_AddImportC(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSmartEdit(registry, tmpDir)

	cFile := filepath.Join(tmpDir, "test.c")
	cContent := `#include <stdio.h>

int main() {
    printf("hello\n");
    return 0;
}
`
	if err := os.WriteFile(cFile, []byte(cContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("smart_edit")
	if !ok {
		t.Fatal("smart_edit tool not registered")
	}

	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":      "test.c",
		"operation": "add_import",
		"value":     "<stdlib.h>",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	newContent, err := os.ReadFile(cFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), "#include <stdlib.h>") {
		t.Errorf("Should add include statement, got: %s", string(newContent))
	}
}

func TestSmartEdit_AddFunction(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSmartEdit(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

func main() {
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("smart_edit")
	if !ok {
		t.Fatal("smart_edit tool not registered")
	}

	newFunc := `func Helper() string {
	return "helper"
}`

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":      "test.go",
		"operation": "add_function",
		"value":     newFunc,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Successfully performed add_function") {
		t.Error("Should indicate success")
	}

	newContent, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), "func Helper()") {
		t.Error("Should add function")
	}
}

func TestSmartEdit_DuplicateImport(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterSmartEdit(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

import "fmt"

func main() {
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("smart_edit")
	if !ok {
		t.Fatal("smart_edit tool not registered")
	}

	// Try to add duplicate import
	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":      "test.go",
		"operation": "add_import",
		"value":     "fmt",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	newContent, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Should not duplicate the import
	count := strings.Count(string(newContent), `"fmt"`)
	if count > 1 {
		t.Errorf("Should not duplicate import, found %d occurrences", count)
	}
}
