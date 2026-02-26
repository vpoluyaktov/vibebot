package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

// OpenRouter implements the Provider interface for OpenRouter
type OpenRouter struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenRouter creates a new OpenRouter provider
func NewOpenRouter(apiKey, model string) *OpenRouter {
	return &OpenRouter{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

// Name returns the provider name
func (o *OpenRouter) Name() string {
	return "openrouter"
}

// openRouterRequest matches OpenRouter's API format
type openRouterRequest struct {
	Model    string                   `json:"model"`
	Messages []map[string]interface{} `json:"messages"` // Use generic map for flexibility
	Tools    []Tool                   `json:"tools,omitempty"`
}

// openRouterResponse matches OpenRouter's API response
type openRouterResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Chat sends messages to OpenRouter and returns the response
func (o *OpenRouter) Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	// Convert messages to generic format
	apiMessages := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		apiMsg := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
		if len(msg.ToolCalls) > 0 {
			apiMsg["tool_calls"] = msg.ToolCalls
		}
		if msg.ToolCallID != "" {
			apiMsg["tool_call_id"] = msg.ToolCallID
		}
		apiMessages[i] = apiMsg
	}

	reqBody := openRouterRequest{
		Model:    o.model,
		Messages: apiMessages,
		Tools:    tools,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openRouterURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/vpoluyaktov/vibebot")
	req.Header.Set("X-Title", "vibebot")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openrouter returned status %d: %s", resp.StatusCode, string(body))
	}

	var orResp openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(orResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := orResp.Choices[0]
	return &Response{
		Content:      choice.Message.Content,
		ToolCalls:    choice.Message.ToolCalls,
		FinishReason: choice.FinishReason,
	}, nil
}
