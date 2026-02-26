package llm

import "context"

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

// ToolCall represents a function call request from the LLM
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

// Response represents an LLM response
type Response struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string  `json:"finish_reason"` // "stop", "tool_calls", etc.
}

// Provider defines the interface for LLM providers
type Provider interface {
	// Chat sends messages and returns a response
	Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error)
	
	// Name returns the provider name
	Name() string
}

// Tool represents a function that can be called by the LLM
type Tool struct {
	Type     string   `json:"type"` // "function"
	Function Function `json:"function"`
}

// Function describes a callable function
type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}
