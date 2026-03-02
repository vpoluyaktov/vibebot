package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	openRouterURL       = "https://openrouter.ai/api/v1/chat/completions"
	openRouterModelsURL = "https://openrouter.ai/api/v1/models"
)

// OpenRouter implements the Provider interface for OpenRouter
type OpenRouter struct {
	apiKey string
	model  string
	client *http.Client
	mu     sync.RWMutex
}

// NewOpenRouter creates a new OpenRouter provider
func NewOpenRouter(apiKey, model string) *OpenRouter {
	return &OpenRouter{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{
			Timeout: 120 * time.Second, // 2 minute timeout for LLM requests
		},
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

// SetModel updates the model name (thread-safe)
func (o *OpenRouter) SetModel(model string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.model = model
}

// GetModel returns the current model name (thread-safe)
func (o *OpenRouter) GetModel() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.model
}

// Chat sends messages to OpenRouter and returns the response
func (o *OpenRouter) Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	// Get current model (thread-safe)
	o.mu.RLock()
	currentModel := o.model
	o.mu.RUnlock()
	
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
		Model:    currentModel,
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
		// Return error as content for graceful handling (matches nanobot behavior)
		return &Response{
			Content:      fmt.Sprintf("⚠️ Network error: %v. Please check your connection and try again.", err),
			FinishReason: "error",
		}, nil
	}
	defer resp.Body.Close()

	// Extract credit usage from X-OpenRouter-Usage header
	var credits float64
	if usageHeader := resp.Header.Get("X-OpenRouter-Usage"); usageHeader != "" {
		// Parse JSON header: {"credits": 0.0015}
		var usageData struct {
			Credits float64 `json:"credits"`
		}
		if err := json.Unmarshal([]byte(usageHeader), &usageData); err == nil {
			credits = usageData.Credits
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Return error as content for graceful handling (matches nanobot behavior)
		return &Response{
			Content:      fmt.Sprintf("⚠️ API error (status %d): %s. This might be due to rate limiting or service issues.", resp.StatusCode, string(body)),
			FinishReason: "error",
		}, nil
	}

	var orResp openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
		// Return error as content for graceful handling (matches nanobot behavior)
		return &Response{
			Content:      fmt.Sprintf("⚠️ Failed to parse API response: %v. The service might be experiencing issues.", err),
			FinishReason: "error",
		}, nil
	}

	if len(orResp.Choices) == 0 {
		// Return error as content for graceful handling (matches nanobot behavior)
		return &Response{
			Content:      "⚠️ The AI model returned an empty response. This usually indicates rate limiting or temporary service issues. Please try again in a moment.",
			FinishReason: "error",
		}, nil
	}

	choice := orResp.Choices[0]
	return &Response{
		Content:      choice.Message.Content,
		ToolCalls:    choice.Message.ToolCalls,
		FinishReason: choice.FinishReason,
		Usage: TokenUsage{
			PromptTokens:     orResp.Usage.PromptTokens,
			CompletionTokens: orResp.Usage.CompletionTokens,
			TotalTokens:      orResp.Usage.TotalTokens,
			Credits:          credits,
		},
	}, nil
}

// ModelInfo represents model metadata from OpenRouter
type ModelInfo struct {
	ID            string `json:"id"`
	ContextLength int    `json:"context_length"`
}

// modelsResponse matches OpenRouter's /models API response
type modelsResponse struct {
	Data []ModelInfo `json:"data"`
}

// FetchModelContextLengths fetches context lengths for given model IDs
func (o *OpenRouter) FetchModelContextLengths(ctx context.Context, modelIDs []string) (map[string]int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", openRouterModelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var modelsResp modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	// Build a map of requested models
	requested := make(map[string]bool)
	for _, id := range modelIDs {
		requested[id] = true
	}

	// Extract context lengths for requested models
	contextLengths := make(map[string]int)
	for _, model := range modelsResp.Data {
		if requested[model.ID] {
			contextLengths[model.ID] = model.ContextLength
		}
	}

	return contextLengths, nil
}
