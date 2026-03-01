package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetLanguageByExtension(t *testing.T) {
	tests := []struct {
		ext      string
		wantLang string
		wantOk   bool
	}{
		{".go", "go", true},
		{".py", "python", true},
		{".js", "javascript", true},
		{".ts", "typescript", true},
		{".java", "java", true},
		{".c", "c", true},
		{".cpp", "cpp", true},
		{".rs", "rust", true},
		{".unknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			lang, ok := GetLanguageByExtension(tt.ext)
			if ok != tt.wantOk {
				t.Errorf("GetLanguageByExtension(%q) ok = %v, want %v", tt.ext, ok, tt.wantOk)
				return
			}
			if ok && lang.Name != tt.wantLang {
				t.Errorf("GetLanguageByExtension(%q) lang = %v, want %v", tt.ext, lang.Name, tt.wantLang)
			}
		})
	}
}

func TestParseGoFile(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

import "fmt"

// User represents a user
type User struct {
	Name string
}

// NewUser creates a user
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

	tree, lang, err := ParseFile(goFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if lang.Name != "go" {
		t.Errorf("ParseFile() lang = %v, want go", lang.Name)
	}
	if tree == nil {
		t.Fatal("ParseFile() returned nil tree")
	}

	// Extract symbols
	symbols := ExtractSymbols(tree, lang, []byte(goContent), false)
	
	// Should have: User type, NewUser function, GetName method (no privateHelper)
	if len(symbols) < 3 {
		t.Errorf("ExtractSymbols() found %d symbols, want at least 3", len(symbols))
	}

	// Check for specific symbols
	foundUser := false
	foundNewUser := false
	foundGetName := false
	foundPrivate := false

	for _, sym := range symbols {
		switch sym.Name {
		case "User":
			foundUser = true
			if sym.Type != "struct" {
				t.Errorf("User type = %v, want struct", sym.Type)
			}
		case "NewUser":
			foundNewUser = true
			if sym.Type != "function" {
				t.Errorf("NewUser type = %v, want function", sym.Type)
			}
		case "GetName":
			foundGetName = true
			if sym.Type != "method" {
				t.Errorf("GetName type = %v, want method", sym.Type)
			}
		case "privateHelper":
			foundPrivate = true
		}
	}

	if !foundUser {
		t.Error("User type not found")
	}
	if !foundNewUser {
		t.Error("NewUser function not found")
	}
	if !foundGetName {
		t.Error("GetName method not found")
	}
	if foundPrivate {
		t.Error("privateHelper should not be included (not exported)")
	}

	// Test with includePrivate = true
	symbolsWithPrivate := ExtractSymbols(tree, lang, []byte(goContent), true)
	foundPrivate = false
	for _, sym := range symbolsWithPrivate {
		if sym.Name == "privateHelper" {
			foundPrivate = true
			break
		}
	}
	if !foundPrivate {
		t.Error("privateHelper should be included when includePrivate=true")
	}
}

func TestParsePythonFile(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test.py")
	pyContent := `import os

class User:
    def __init__(self, name):
        self.name = name
    
    def get_name(self):
        return self.name
    
    def _private_method(self):
        pass

def public_function():
    pass

def _private_function():
    pass
`
	if err := os.WriteFile(pyFile, []byte(pyContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tree, lang, err := ParseFile(pyFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if lang.Name != "python" {
		t.Errorf("ParseFile() lang = %v, want python", lang.Name)
	}

	symbols := ExtractSymbols(tree, lang, []byte(pyContent), false)
	
	// Should have: User class, public_function (no private ones)
	foundUser := false
	foundPublic := false
	foundPrivateFunc := false

	for _, sym := range symbols {
		switch sym.Name {
		case "User":
			foundUser = true
		case "public_function":
			foundPublic = true
		case "_private_function":
			foundPrivateFunc = true
		}
	}

	if !foundUser {
		t.Error("User class not found")
	}
	if !foundPublic {
		t.Error("public_function not found")
	}
	if foundPrivateFunc {
		t.Error("_private_function should not be included")
	}
}

func TestParseJavaScriptFile(t *testing.T) {
	tmpDir := t.TempDir()
	jsFile := filepath.Join(tmpDir, "test.js")
	jsContent := `class User {
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

const helper = () => {
    return 'helper';
};
`
	if err := os.WriteFile(jsFile, []byte(jsContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tree, lang, err := ParseFile(jsFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if lang.Name != "javascript" {
		t.Errorf("ParseFile() lang = %v, want javascript", lang.Name)
	}

	symbols := ExtractSymbols(tree, lang, []byte(jsContent), false)
	
	foundUser := false
	foundCreate := false

	for _, sym := range symbols {
		switch sym.Name {
		case "User":
			foundUser = true
		case "createUser":
			foundCreate = true
		}
	}

	if !foundUser {
		t.Error("User class not found")
	}
	if !foundCreate {
		t.Error("createUser function not found")
	}
}

func TestParseJavaFile(t *testing.T) {
	tmpDir := t.TempDir()
	javaFile := filepath.Join(tmpDir, "Test.java")
	javaContent := `package com.example;

public class User {
    private String name;
    
    public User(String name) {
        this.name = name;
    }
    
    public String getName() {
        return name;
    }
    
    private void privateMethod() {
    }
}
`
	if err := os.WriteFile(javaFile, []byte(javaContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tree, lang, err := ParseFile(javaFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if lang.Name != "java" {
		t.Errorf("ParseFile() lang = %v, want java", lang.Name)
	}

	symbols := ExtractSymbols(tree, lang, []byte(javaContent), false)
	
	foundUser := false
	foundGetName := false
	foundPrivate := false

	for _, sym := range symbols {
		switch sym.Name {
		case "User":
			foundUser = true
			if !sym.IsExported {
				t.Error("User class should be exported (public)")
			}
		case "getName":
			foundGetName = true
		case "privateMethod":
			foundPrivate = true
		}
	}

	if !foundUser {
		t.Error("User class not found")
	}
	if !foundGetName {
		t.Error("getName method not found")
	}
	if foundPrivate {
		t.Error("privateMethod should not be included (not public)")
	}
}

func TestGetImports(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
		want     []string
	}{
		{
			name:     "Go imports",
			filename: "test.go",
			content: `package main

import (
	"fmt"
	"os"
)`,
			want: []string{"fmt", "os"},
		},
		{
			name:     "Python imports",
			filename: "test.py",
			content: `import os
import sys
from typing import List`,
			want: []string{"import os", "import sys", "from typing import List"},
		},
		{
			name:     "JavaScript imports",
			filename: "test.js",
			content: `import React from 'react';
import { useState } from 'react';`,
			want: []string{"import React from 'react';", "import { useState } from 'react';"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			tree, lang, err := ParseFile(filePath)
			if err != nil {
				t.Fatalf("ParseFile() error = %v", err)
			}

			imports := GetImports(tree, lang, []byte(tt.content))
			if len(imports) != len(tt.want) {
				t.Errorf("GetImports() found %d imports, want %d", len(imports), len(tt.want))
			}
		})
	}
}
