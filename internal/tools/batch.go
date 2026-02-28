package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
)

// BatchToolCall represents a single tool call in a batch
type BatchToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// RegisterBatchTools adds the batch_tools capability to the registry
func RegisterBatchTools(registry *Registry) {
	registry.Register("batch_tools", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "batch_tools",
				Description: "Execute multiple tool calls in a single request. Reduces LLM round-trips for multi-step operations. Use this when you need to perform multiple consecutive operations (e.g., edit multiple files, read and write, etc.).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"calls": map[string]interface{}{
							"type":        "array",
							"description": "Array of tool calls to execute in sequence",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{
										"type":        "string",
										"description": "Tool name to execute",
									},
									"arguments": map[string]interface{}{
										"type":        "object",
										"description": "Tool parameters as key-value pairs",
									},
								},
								"required": []string{"name", "arguments"},
							},
						},
					},
					"required": []string{"calls"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			// Extract calls array
			callsRaw, ok := args["calls"]
			if !ok {
				return "", fmt.Errorf("calls parameter is required")
			}

			callsArray, ok := callsRaw.([]interface{})
			if !ok {
				return "", fmt.Errorf("calls must be an array")
			}

			if len(callsArray) == 0 {
				return "", fmt.Errorf("calls array cannot be empty")
			}

			// Parse batch calls
			var batchCalls []BatchToolCall
			for i, callRaw := range callsArray {
				callMap, ok := callRaw.(map[string]interface{})
				if !ok {
					return "", fmt.Errorf("call %d is not a valid object", i)
				}

				name, ok := callMap["name"].(string)
				if !ok {
					return "", fmt.Errorf("call %d: name must be a string", i)
				}

				arguments, ok := callMap["arguments"].(map[string]interface{})
				if !ok {
					return "", fmt.Errorf("call %d: arguments must be an object", i)
				}

				batchCalls = append(batchCalls, BatchToolCall{
					Name:      name,
					Arguments: arguments,
				})
			}

			logger.Debug("Executing batch_tools with %d calls", len(batchCalls))

			// Execute each tool call in sequence
			var results []string
			for i, call := range batchCalls {
				logger.Debug("Batch call %d/%d: %s", i+1, len(batchCalls), call.Name)

				// Get the tool from registry
				tool, exists := registry.Get(call.Name)
				if !exists {
					errMsg := fmt.Sprintf("Tool '%s' not found", call.Name)
					results = append(results, fmt.Sprintf("[%d] %s: ERROR - %s", i+1, call.Name, errMsg))
					logger.Error("Batch call %d failed: %s", i+1, errMsg)
					continue
				}

				// Execute the tool
				result, err := tool.Handler(ctx, call.Arguments)
				if err != nil {
					results = append(results, fmt.Sprintf("[%d] %s: ERROR - %v", i+1, call.Name, err))
					logger.Error("Batch call %d (%s) failed: %v", i+1, call.Name, err)
				} else {
					// Truncate long results for readability
					truncatedResult := result
					if len(result) > 500 {
						truncatedResult = result[:500] + fmt.Sprintf("... (%d more bytes)", len(result)-500)
					}
					results = append(results, fmt.Sprintf("[%d] %s: SUCCESS - %s", i+1, call.Name, truncatedResult))
					logger.Debug("Batch call %d (%s) succeeded", i+1, call.Name)
				}
			}

			// Format combined results
			var output strings.Builder
			output.WriteString(fmt.Sprintf("Batch execution completed: %d/%d successful\n\n", countSuccessful(results), len(results)))
			for _, result := range results {
				output.WriteString(result)
				output.WriteString("\n")
			}

			return output.String(), nil
		},
	})
}

// countSuccessful counts how many results contain SUCCESS
func countSuccessful(results []string) int {
	count := 0
	for _, result := range results {
		if strings.Contains(result, "SUCCESS") {
			count++
		}
	}
	return count
}

// ExecuteBatch is a helper function that can be called directly from the agent
// to execute a batch of tool calls without going through the tool registry
func ExecuteBatch(ctx context.Context, registry *Registry, toolCalls []llm.ToolCall) ([]string, error) {
	results := make([]string, len(toolCalls))

	for i, toolCall := range toolCalls {
		logger.Debug("Executing batch tool %d/%d: %s", i+1, len(toolCalls), toolCall.Function.Name)

		result, err := registry.Execute(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
		if err != nil {
			results[i] = fmt.Sprintf("Error: %v", err)
			logger.Error("Batch tool execution error: %v", err)
		} else {
			results[i] = result
		}
	}

	return results, nil
}
