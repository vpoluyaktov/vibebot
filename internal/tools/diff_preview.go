package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
)

func RegisterDiffPreview(registry *Registry, workspaceDir string) {
	registry.Register("diff_preview", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "diff_preview",
				Description: "Preview what changes would look like before applying them. Helps verify edits are correct without executing them.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"edits": map[string]interface{}{
							"type":        "array",
							"description": "Array of edit operations to preview",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"path": map[string]interface{}{
										"type":        "string",
										"description": "File path",
									},
									"old_text": map[string]interface{}{
										"type":        "string",
										"description": "Text to find",
									},
									"new_text": map[string]interface{}{
										"type":        "string",
										"description": "Text to replace with",
									},
								},
								"required": []string{"path", "old_text", "new_text"},
							},
						},
					},
					"required": []string{"edits"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			editsRaw, ok := args["edits"]
			if !ok {
				return "", fmt.Errorf("edits parameter is required")
			}

			editsArray, ok := editsRaw.([]interface{})
			if !ok {
				return "", fmt.Errorf("edits must be an array")
			}

			if len(editsArray) == 0 {
				return "", fmt.Errorf("edits array cannot be empty")
			}

			var previews []string
			validCount := 0

			for i, editRaw := range editsArray {
				editMap, ok := editRaw.(map[string]interface{})
				if !ok {
					previews = append(previews, fmt.Sprintf("[%d] ERROR: invalid edit object", i+1))
					continue
				}

				path, _ := editMap["path"].(string)
				oldText, _ := editMap["old_text"].(string)
				newText, _ := editMap["new_text"].(string)

				if path == "" || oldText == "" {
					previews = append(previews, fmt.Sprintf("[%d] ERROR: path and old_text are required", i+1))
					continue
				}

				// Resolve path
				if !filepath.IsAbs(path) {
					path = filepath.Join(workspaceDir, path)
				}

				// Read file
				data, err := os.ReadFile(path)
				if err != nil {
					previews = append(previews, fmt.Sprintf("[%d] %s: ERROR - %v", i+1, editMap["path"], err))
					continue
				}

				content := string(data)

				// Check if old_text exists
				if !strings.Contains(content, oldText) {
					previews = append(previews, fmt.Sprintf("[%d] %s: ERROR - old_text not found in file", i+1, editMap["path"]))
					continue
				}

				// Generate diff preview
				newContent := strings.Replace(content, oldText, newText, 1)

				var preview strings.Builder
				preview.WriteString(fmt.Sprintf("[%d] %s:\n", i+1, editMap["path"]))
				preview.WriteString("--- Before:\n")
				preview.WriteString(truncateForPreview(oldText))
				preview.WriteString("\n+++ After:\n")
				preview.WriteString(truncateForPreview(newText))
				preview.WriteString(fmt.Sprintf("\n(File size: %d → %d bytes)", len(content), len(newContent)))

				previews = append(previews, preview.String())
				validCount++
			}

			var output strings.Builder
			output.WriteString(fmt.Sprintf("Preview of %d/%d edits:\n\n", validCount, len(editsArray)))
			for _, preview := range previews {
				output.WriteString(preview)
				output.WriteString("\n---\n")
			}

			logger.Debug("diff_preview: previewed %d/%d edits", validCount, len(editsArray))
			return output.String(), nil
		},
	})
}

func truncateForPreview(text string) string {
	if len(text) > 200 {
		return text[:200] + "...\n(truncated)"
	}
	return text
}
