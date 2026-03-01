package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSummary_GoFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test Go file
	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package main

import (
	"fmt"
	"os"
)

// User represents a user in the system
type User struct {
	ID   int
	Name string
}

// Config holds application configuration
type Config interface {
	GetValue(key string) string
}

const MaxRetries = 3

var DefaultTimeout = 30

// NewUser creates a new user
func NewUser(id int, name string) *User {
	return &User{ID: id, Name: name}
}

// GetName returns the user's name
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

	registry := NewRegistry()
	RegisterFileSummary(registry, tmpDir)

	tests := []struct {
		name           string
		args           map[string]interface{}
		wantContains   []string
		wantNotContain []string
		wantErr        bool
	}{
		{
			name: "basic summary without private",
			args: map[string]interface{}{
				"path":             "test.go",
				"include_private":  false,
				"include_comments": true,
			},
			wantContains: []string{
				"Package: main",
				"fmt",
				"os",
				"type User struct",
				"type Config interface",
				"const MaxRetries",
				"var DefaultTimeout",
				"func NewUser",
				"func (*User) GetName",
				"User represents a user",
			},
			wantNotContain: []string{
				"privateHelper",
			},
		},
		{
			name: "include private symbols",
			args: map[string]interface{}{
				"path":             "test.go",
				"include_private":  true,
				"include_comments": false,
			},
			wantContains: []string{
				"Package: main",
				"type User struct",
				"func NewUser",
				"func privateHelper",
			},
			wantNotContain: []string{
				"User represents",
			},
		},
		{
			name: "exclude comments",
			args: map[string]interface{}{
				"path":             "test.go",
				"include_private":  false,
				"include_comments": false,
			},
			wantContains: []string{
				"Package: main",
				"type User struct",
			},
			wantNotContain: []string{
				"User represents",
				"Config holds",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := registry.Get("file_summary")
			if !ok {
				t.Fatal("file_summary tool not registered")
			}

			result, err := tool.Handler(context.Background(), tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Handler() error = %v, wantErr %v", err, tt.wantErr)
				return
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

func TestFileSummary_PythonFile(t *testing.T) {
	tmpDir := t.TempDir()

	pyFile := filepath.Join(tmpDir, "test.py")
	pyContent := `import os
import sys
from typing import List

class User:
    def __init__(self, name):
        self.name = name
    
    def get_name(self):
        return self.name
    
    def _private_method(self):
        pass

def public_function(x, y):
    return x + y

def _private_function():
    pass
`
	if err := os.WriteFile(pyFile, []byte(pyContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	registry := NewRegistry()
	RegisterFileSummary(registry, tmpDir)

	tests := []struct {
		name           string
		args           map[string]interface{}
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "python without private",
			args: map[string]interface{}{
				"path":            "test.py",
				"include_private": false,
			},
			wantContains: []string{
				"import os",
				"import sys",
				"from typing import List",
				"class User",
				"def public_function",
			},
			wantNotContain: []string{
				"_private_function",
				"_private_method",
			},
		},
		{
			name: "python with private",
			args: map[string]interface{}{
				"path":            "test.py",
				"include_private": true,
			},
			wantContains: []string{
				"class User",
				"def public_function",
				"def _private_function",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := registry.Get("file_summary")
			if !ok {
				t.Fatal("file_summary tool not registered")
			}

			result, err := tool.Handler(context.Background(), tt.args)
			if err != nil {
				t.Errorf("Handler() error = %v", err)
				return
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

func TestFileSummary_JavaScriptFile(t *testing.T) {
	tmpDir := t.TempDir()

	jsFile := filepath.Join(tmpDir, "test.js")
	jsContent := `import React from 'react';
import { useState } from 'react';

export class User {
    constructor(name) {
        this.name = name;
    }
}

export function createUser(name) {
    return new User(name);
}

const helper = () => {
    return 'helper';
};

function privateFunc() {
    console.log('private');
}
`
	if err := os.WriteFile(jsFile, []byte(jsContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	registry := NewRegistry()
	RegisterFileSummary(registry, tmpDir)

	tool, ok := registry.Get("file_summary")
	if !ok {
		t.Fatal("file_summary tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": "test.js",
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	wantContains := []string{
		"import React",
		"import { useState }",
		"export class User",
		"export function createUser",
	}

	for _, want := range wantContains {
		if !strings.Contains(result, want) {
			t.Errorf("Result missing expected content: %q\nGot:\n%s", want, result)
		}
	}
}

func TestFileSummary_GenericFile(t *testing.T) {
	tmpDir := t.TempDir()

	txtFile := filepath.Join(tmpDir, "test.txt")
	txtContent := `Line 1
Line 2

Line 4
Line 5
`
	if err := os.WriteFile(txtFile, []byte(txtContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	registry := NewRegistry()
	RegisterFileSummary(registry, tmpDir)

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

	wantContains := []string{
		"File: test.txt",
		"Size:",
		"Lines: 5",
		"non-empty: 4",
		"Type: .txt",
	}

	for _, want := range wantContains {
		if !strings.Contains(result, want) {
			t.Errorf("Result missing expected content: %q\nGot:\n%s", want, result)
		}
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

func TestFileSummary_ComplexGoFile(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "complex.go")
	goContent := `package tools

import (
	"context"
	"fmt"
)

// Registry manages tools
type Registry struct {
	tools map[string]*Tool
}

// Tool represents a tool
type Tool struct {
	Definition interface{}
	Handler    func(ctx context.Context, args map[string]interface{}) (string, error)
}

// NewRegistry creates a registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*Tool),
	}
}

// Register adds a tool
func (r *Registry) Register(name string, tool *Tool) {
	r.tools[name] = tool
}

// Get retrieves a tool
func (r *Registry) Get(name string) (*Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	registry := NewRegistry()
	RegisterFileSummary(registry, tmpDir)

	tool, ok := registry.Get("file_summary")
	if !ok {
		t.Fatal("file_summary tool not registered")
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":             "complex.go",
		"include_comments": true,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	wantContains := []string{
		"Package: tools",
		"type Registry struct { 1 fields }",
		"type Tool struct { 2 fields }",
		"func NewRegistry",
		"func (*Registry) Register",
		"func (*Registry) Get",
		"Registry manages tools",
		"Tool represents a tool",
	}

	for _, want := range wantContains {
		if !strings.Contains(result, want) {
			t.Errorf("Result missing expected content: %q\nGot:\n%s", want, result)
		}
	}
}

func TestFileSummary_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()

	goFile := filepath.Join(tmpDir, "abs.go")
	goContent := `package main

func main() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	registry := NewRegistry()
	RegisterFileSummary(registry, tmpDir)

	tool, ok := registry.Get("file_summary")
	if !ok {
		t.Fatal("file_summary tool not registered")
	}

	// Test with absolute path
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": goFile,
	})
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	if !strings.Contains(result, "Package: main") {
		t.Errorf("Result missing package declaration: %s", result)
	}
}
