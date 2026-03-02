package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
)

// SearchCache stores search results with TTL
type SearchCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
}

type CacheEntry struct {
	Results   []SearchResult
	Timestamp time.Time
	Query     string
}

type SearchResult struct {
	FilePath string
	LineNum  int
	Line     string
	Context  []string // Surrounding lines
}

var globalSearchCache = &SearchCache{
	entries: make(map[string]*CacheEntry),
}

const cacheTTL = 5 * time.Minute

func RegisterCachedGrep(registry *Registry, workspaceDir string) {
	registry.Register("cached_grep", &Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        "cached_grep",
				Description: "Search for text patterns in files. Returns line numbers and context snippets. Caches results for 5 minutes.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Text pattern to search for",
						},
						"file_pattern": map[string]interface{}{
							"type":        "string",
							"description": "Optional file pattern (e.g., '*.go', '*.py')",
						},
						"context_lines": map[string]interface{}{
							"type":        "integer",
							"description": "Number of context lines before/after match (default: 2)",
							"default":     2,
						},
						"max_results": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of results to return (default: 50)",
							"default":     50,
						},
						"case_sensitive": map[string]interface{}{
							"type":        "boolean",
							"description": "Case-sensitive search (default: false)",
							"default":     false,
						},
					},
					"required": []string{"query"},
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			query, ok := args["query"].(string)
			if !ok {
				return "", fmt.Errorf("query must be a string")
			}

			filePattern := ""
			if pattern, ok := args["file_pattern"].(string); ok {
				filePattern = pattern
			}

			contextLines := 2
			if ctx, ok := args["context_lines"].(float64); ok {
				contextLines = int(ctx)
			}

			maxResults := 50
			if max, ok := args["max_results"].(float64); ok {
				maxResults = int(max)
			}

			caseSensitive := false
			if cs, ok := args["case_sensitive"].(bool); ok {
				caseSensitive = cs
			}

			// Generate cache key
			cacheKey := generateCacheKey(workspaceDir, query, filePattern, caseSensitive)

			// Check cache
			if cached := getCachedResults(cacheKey); cached != nil {
				logger.Debug("cached_grep: cache hit for query '%s'", query)
				return formatResults(cached.Results, query, maxResults, true), nil
			}

			// Perform search
			results, err := performSearch(workspaceDir, query, filePattern, contextLines, caseSensitive)
			if err != nil {
				return "", fmt.Errorf("search failed: %w", err)
			}

			// Cache results
			cacheResults(cacheKey, query, results)

			logger.Debug("cached_grep: found %d matches for '%s'", len(results), query)
			return formatResults(results, query, maxResults, false), nil
		},
	})
}

func generateCacheKey(workspace, query, filePattern string, caseSensitive bool) string {
	data := fmt.Sprintf("%s|%s|%s|%v", workspace, query, filePattern, caseSensitive)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:16])
}

func getCachedResults(key string) *CacheEntry {
	globalSearchCache.mu.RLock()
	defer globalSearchCache.mu.RUnlock()

	entry, exists := globalSearchCache.entries[key]
	if !exists {
		return nil
	}

	// Check if cache is still valid
	if time.Since(entry.Timestamp) > cacheTTL {
		return nil
	}

	return entry
}

func cacheResults(key, query string, results []SearchResult) {
	globalSearchCache.mu.Lock()
	defer globalSearchCache.mu.Unlock()

	globalSearchCache.entries[key] = &CacheEntry{
		Results:   results,
		Timestamp: time.Now(),
		Query:     query,
	}

	// Clean up old entries
	for k, v := range globalSearchCache.entries {
		if time.Since(v.Timestamp) > cacheTTL {
			delete(globalSearchCache.entries, k)
		}
	}
}

func performSearch(workspaceDir, query, filePattern string, contextLines int, caseSensitive bool) ([]SearchResult, error) {
	var results []SearchResult
	searchQuery := query
	if !caseSensitive {
		searchQuery = strings.ToLower(query)
	}

	err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			// Skip common directories
			if info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check file pattern
		if filePattern != "" {
			matched, _ := filepath.Match(filePattern, filepath.Base(path))
			if !matched {
				return nil
			}
		}

		// Skip binary files (simple heuristic)
		if isBinaryFile(path) {
			return nil
		}

		// Search in file
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			searchLine := line
			if !caseSensitive {
				searchLine = strings.ToLower(line)
			}

			if strings.Contains(searchLine, searchQuery) {
				relPath, _ := filepath.Rel(workspaceDir, path)

				// Get context lines
				var context []string
				start := i - contextLines
				if start < 0 {
					start = 0
				}
				end := i + contextLines + 1
				if end > len(lines) {
					end = len(lines)
				}

				for j := start; j < end; j++ {
					if j == i {
						context = append(context, fmt.Sprintf("> %4d | %s", j+1, lines[j]))
					} else {
						context = append(context, fmt.Sprintf("  %4d | %s", j+1, lines[j]))
					}
				}

				results = append(results, SearchResult{
					FilePath: relPath,
					LineNum:  i + 1,
					Line:     line,
					Context:  context,
				})
			}
		}

		return nil
	})

	return results, err
}

func isBinaryFile(path string) bool {
	// Simple heuristic: check extension
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := []string{
		".exe", ".dll", ".so", ".dylib", ".a", ".o",
		".png", ".jpg", ".jpeg", ".gif", ".ico", ".bmp",
		".zip", ".tar", ".gz", ".bz2", ".7z",
		".pdf", ".doc", ".docx", ".xls", ".xlsx",
		".mp3", ".mp4", ".avi", ".mov",
	}

	for _, binExt := range binaryExts {
		if ext == binExt {
			return true
		}
	}

	return false
}

func formatResults(results []SearchResult, query string, maxResults int, fromCache bool) string {
	var output strings.Builder

	if fromCache {
		output.WriteString(fmt.Sprintf("Found %d matches for '%s' (from cache):\n\n", len(results), query))
	} else {
		output.WriteString(fmt.Sprintf("Found %d matches for '%s':\n\n", len(results), query))
	}

	if len(results) == 0 {
		output.WriteString("No matches found.\n")
		return output.String()
	}

	// Limit results
	displayCount := len(results)
	if displayCount > maxResults {
		displayCount = maxResults
	}

	for i := 0; i < displayCount; i++ {
		result := results[i]
		output.WriteString(fmt.Sprintf("[%d] %s:%d\n", i+1, result.FilePath, result.LineNum))
		for _, contextLine := range result.Context {
			output.WriteString(contextLine + "\n")
		}
		output.WriteString("\n")
	}

	if len(results) > maxResults {
		output.WriteString(fmt.Sprintf("... and %d more matches (use max_results to see more)\n", len(results)-maxResults))
	}

	return output.String()
}
