package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vpoluyaktov/vibebot/internal/llm"
)

// Handler is a function that executes a tool
type Handler func(ctx context.Context, args map[string]interface{}) (string, error)

// Tool represents a registered tool
type Tool struct {
	Definition llm.Tool
	Handler    Handler
}

// Registry manages available tools
type Registry struct {
	tools map[string]*Tool
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*Tool),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(name string, tool *Tool) {
	r.tools[name] = tool
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (*Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// GetDefinitions returns all tool definitions for LLM
func (r *Registry) GetDefinitions() []llm.Tool {
	definitions := make([]llm.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		definitions = append(definitions, tool.Definition)
	}
	return definitions
}

// Execute runs a tool with the given arguments
func (r *Registry) Execute(ctx context.Context, name string, argsJSON string) (string, error) {
	tool, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}

	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Execute the tool
	return tool.Handler(ctx, args)
}
