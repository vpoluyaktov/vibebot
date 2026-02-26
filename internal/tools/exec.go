package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/vpoluyaktov/vibebot/internal/llm"
)

const (
	maxOutputSize = 10000 // Maximum output size in characters
	execTimeout   = 60 * time.Second
)

// Dangerous commands that should be blocked
var dangerousCommands = []string{
	"rm -rf",
	"mkfs",
	"dd if=",
	"format",
	"shutdown",
	"reboot",
	"halt",
	"poweroff",
	"init 0",
	"init 6",
	":(){ :|:& };:", // fork bomb
}

// RegisterExecTool adds the exec tool to the registry
func RegisterExecTool(registry *Registry, workspaceDir string) {
	registry.Register("exec", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "exec",
				Description: "Execute a shell command and return its output. Use with caution. Timeout: 60s, output truncated at 10k chars.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "The shell command to execute",
						},
						"working_dir": map[string]interface{}{
							"type":        "string",
							"description": "Optional working directory for the command",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			command, ok := args["command"].(string)
			if !ok {
				return "", fmt.Errorf("command must be a string")
			}

			// Check for dangerous commands
			commandLower := strings.ToLower(command)
			for _, dangerous := range dangerousCommands {
				if strings.Contains(commandLower, dangerous) {
					return "", fmt.Errorf("dangerous command blocked: %s", dangerous)
				}
			}

			// Get working directory
			workingDir := workspaceDir
			if wd, ok := args["working_dir"].(string); ok && wd != "" {
				workingDir = wd
			}

			// Create command with timeout
			ctx, cancel := context.WithTimeout(ctx, execTimeout)
			defer cancel()

			cmd := exec.CommandContext(ctx, "bash", "-c", command)
			cmd.Dir = workingDir

			// Run command
			output, err := cmd.CombinedOutput()
			
			// Truncate output if too large
			result := string(output)
			if len(result) > maxOutputSize {
				result = result[:maxOutputSize] + "\n... (output truncated)"
			}

			// Format result
			var resultBuilder strings.Builder
			if err != nil {
				resultBuilder.WriteString(fmt.Sprintf("Command failed: %v\n", err))
			}
			if len(result) > 0 {
				// Separate stdout and stderr if possible
				if strings.Contains(result, "STDERR:") {
					resultBuilder.WriteString(result)
				} else {
					resultBuilder.WriteString(result)
				}
			}

			return resultBuilder.String(), nil
		},
	})
}
