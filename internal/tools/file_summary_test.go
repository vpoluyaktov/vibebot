package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSummary_MultiLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileSummary(registry, tmpDir)

	tests := []struct {
		name         string
		filename     string
		content      string
		wantContains []string
		wantNotContain []string
		includePrivate bool
	}{
		{
			name:     "Go file",
			filename: "test.go",
			content: `package main

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
`,
			wantContains: []string{
				"Language: go",
				"fmt",
				"type User",
				"func NewUser",
				"GetName",
			},
			wantNotContain: []string{
				"privateHelper",
			},
			includePrivate: false,
		},
		{
			name:     "Python file",
			filename: "test.py",
			content: `import os
import sys

class User:
    def __init__(self, name):
        self.name = name
    
    def get_name(self):
        return self.name

def create_user(name):
    return User(name)

def _private_helper():
    pass
`,
			wantContains: []string{
				"Language: python",
				"import os",
				"import sys",
				"class User",
				"def create_user",
			},
			wantNotContain: []string{
				"_private_helper",
			},
			includePrivate: false,
		},
		{
			name:     "JavaScript file",
			filename: "test.js",
			content: `import React from 'react';

class User {
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

const helper = () => 'helper';
`,
			wantContains: []string{
				"Language: javascript",
				"import React",
				"class User",
				"function createUser",
			},
			includePrivate: false,
		},
		{
			name:     "Java file",
			filename: "User.java",
			content: `package com.example;

import java.util.List;

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
`,
			wantContains: []string{
				"Language: java",
				"import java.util.List",
				"class User",
				"getName",
			},
			wantNotContain: []string{
				"privateMethod",
			},
			includePrivate: false,
		},
		{
			name:     "C file",
			filename: "test.c",
			content: `#include <stdio.h>

struct User {
    char* name;
    int age;
};

void print_user(struct User* user) {
    printf("%s\n", user->name);
}

int main() {
    return 0;
}
`,
			wantContains: []string{
				"Language: c",
				"struct User",
				"print_user",
				"main",
			},
			includePrivate: false,
		},
		{
			name:     "Rust file",
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
    
    fn private_helper(&self) {
    }
}

pub fn create_user(name: String) -> User {
    User::new(name)
}

fn private_function() {
}
`,
			wantContains: []string{
				"Language: rust",
				"struct User",
				"fn new",
				"fn get_name",
				"fn create_user",
			},
			wantNotContain: []string{
				"private_helper",
				"private_function",
			},
			includePrivate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			tool, ok := registry.Get("file_summary")
			if !ok {
				t.Fatal("file_summary tool not registered")
			}

			result, err := tool.Handler(context.Background(), map[string]interface{}{
				"path":            tt.filename,
				"include_private": tt.includePrivate,
			})
			if err != nil {
				t.Fatalf("Handler() error = %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("Result missing expected content: %q\nGot:\n%s", want, result)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(result, notWant) {
					t.Errorf("Result contains unexpected content: %q\nGot:\n%s", notWant, result)
				}
			}
		})
	}
}

func TestFileSummary_IncludePrivate(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileSummary(registry, tmpDir)

	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

func PublicFunc() {}
func privateFunc() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("file_summary")
	if !ok {
		t.Fatal("file_summary tool not registered")
	}

	// Test without private
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":            "test.go",
		"include_private": false,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "PublicFunc") {
		t.Error("PublicFunc should be included")
	}
	if strings.Contains(result, "privateFunc") {
		t.Error("privateFunc should not be included when include_private=false")
	}

	// Test with private
	result, err = tool.Handler(context.Background(), map[string]interface{}{
		"path":            "test.go",
		"include_private": true,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "PublicFunc") {
		t.Error("PublicFunc should be included")
	}
	if !strings.Contains(result, "privateFunc") {
		t.Error("privateFunc should be included when include_private=true")
	}
}

func TestFileSummary_UnsupportedLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileSummary(registry, tmpDir)

	txtFile := filepath.Join(tmpDir, "test.txt")
	txtContent := `This is a text file
with some content
`
	if err := os.WriteFile(txtFile, []byte(txtContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool, ok := registry.Get("file_summary")
	if !ok {
		t.Fatal("file_summary tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": "test.txt",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	// Should fall back to generic summary
	if !strings.Contains(result, "File: test.txt") {
		t.Error("Generic summary should include filename")
	}
	if !strings.Contains(result, "Size:") {
		t.Error("Generic summary should include size")
	}
}

func TestFileSummary_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileSummary(registry, tmpDir)

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
			tool, ok := registry.Get("file_summary")
			if !ok {
				t.Fatal("file_summary tool not registered")
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

func TestFileSummary_LineNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	RegisterFileSummary(registry, tmpDir)

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

	tool, ok := registry.Get("file_summary")
	if !ok {
		t.Fatal("file_summary tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": "test.go",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	// Check that line numbers are included
	if !strings.Contains(result, "line 5") {
		t.Error("Should include line number for FirstFunc")
	}
	if !strings.Contains(result, "line 9") {
		t.Error("Should include line number for SecondFunc")
	}
}
