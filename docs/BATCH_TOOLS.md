# Batch Tools Optimization

## Overview

The `batch_tools` feature allows the LLM to execute multiple tool calls in a single request, dramatically reducing the number of LLM round-trips required for multi-step operations.

## Problem Statement

Previously, the agent required one LLM call per tool execution:
```
[LLM call] → edit_file → [LLM call] → read_file → [LLM call] → write_file
```

This resulted in:
- **8-12 LLM calls** for file refactoring tasks
- **15+ LLM calls** for code generation + testing
- **5-7 LLM calls** for documentation updates

## Solution

With `batch_tools`, multiple operations are batched into a single LLM decision:
```
[LLM call] → batch_tools([edit_file, read_file, write_file])
```

## Usage

### Tool Definition

```json
{
  "name": "batch_tools",
  "description": "Execute multiple tool calls in a single request. Reduces LLM round-trips for multi-step operations.",
  "parameters": {
    "calls": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": {"type": "string", "description": "Tool name"},
          "arguments": {"type": "object", "description": "Tool parameters"}
        },
        "required": ["name", "arguments"]
      }
    }
  }
}
```

### Example: Multiple File Edits

Instead of making 3 separate tool calls:
```json
// Call 1
{"name": "edit_file", "arguments": {"path": "main.go", "old_text": "...", "new_text": "..."}}

// Call 2
{"name": "edit_file", "arguments": {"path": "utils.go", "old_text": "...", "new_text": "..."}}

// Call 3
{"name": "edit_file", "arguments": {"path": "config.go", "old_text": "...", "new_text": "..."}}
```

Use batch_tools:
```json
{
  "name": "batch_tools",
  "arguments": {
    "calls": [
      {
        "name": "edit_file",
        "arguments": {"path": "main.go", "old_text": "...", "new_text": "..."}
      },
      {
        "name": "edit_file",
        "arguments": {"path": "utils.go", "old_text": "...", "new_text": "..."}
      },
      {
        "name": "edit_file",
        "arguments": {"path": "config.go", "old_text": "...", "new_text": "..."}
      }
    ]
  }
}
```

### Example: Read-Modify-Write Pattern

```json
{
  "name": "batch_tools",
  "arguments": {
    "calls": [
      {
        "name": "read_file",
        "arguments": {"path": "config.yaml"}
      },
      {
        "name": "edit_file",
        "arguments": {"path": "config.yaml", "old_text": "debug: false", "new_text": "debug: true"}
      },
      {
        "name": "exec",
        "arguments": {"command": "go test ./..."}
      }
    ]
  }
}
```

## Expected Impact

| Scenario | Before | After | Improvement |
|----------|--------|-------|-------------|
| File refactor | 8-12 calls | 1-2 calls | **83-92% reduction** |
| Code generation + test | 15+ calls | 3-4 calls | **73-80% reduction** |
| Documentation update | 5-7 calls | 1 call | **80-86% reduction** |

## Implementation Details

### Tool Registration

The batch_tools capability is registered in `cmd/vibebot/main.go`:

```go
toolRegistry := tools.NewRegistry()
tools.RegisterFileTools(toolRegistry, cfg.WorkspaceDir)
tools.RegisterExecTool(toolRegistry, cfg.WorkspaceDir)
tools.RegisterBatchTools(toolRegistry)  // Batch tools registration
```

### Execution Flow

1. LLM decides to use `batch_tools` with multiple operations
2. Agent receives single tool call with array of sub-calls
3. Each sub-call is executed sequentially
4. Results are aggregated and returned
5. LLM processes all results in next iteration

### Error Handling

- Individual tool failures don't stop batch execution
- Each result is marked as SUCCESS or ERROR
- Summary shows `N/M successful` where N is successful count, M is total
- Long results are truncated to 500 bytes for readability

### System Prompt Integration

The LLM is taught to use batch_tools through the system prompt:

```
- **Use batch_tools for multi-step operations** - When you need to perform 
  multiple consecutive operations (e.g., editing multiple files, reading and 
  writing, running multiple commands), use batch_tools to execute them in a 
  single LLM call. This dramatically reduces round-trips.
```

## Benefits

1. **Reduced Latency**: Fewer LLM round-trips = faster task completion
2. **Lower Costs**: Fewer API calls = reduced OpenRouter costs
3. **Better UX**: Users see faster responses
4. **Natural Workflow**: Aligns with how coding tasks are actually structured
5. **Minimal Implementation**: Reuses existing tools, no major refactoring needed

## Limitations

- Tools are executed sequentially (not in parallel)
- Each tool in the batch must be independent (no cross-dependencies)
- Error in one tool doesn't stop others (by design)
- Results are truncated for very long outputs

## Testing

Run the test suite:
```bash
go test -v ./internal/tools -run TestBatchTools
```

Tests cover:
- ✅ Multiple successful operations
- ✅ Partial failures (some succeed, some fail)
- ✅ Invalid tool names
- ✅ File creation and reading
- ✅ Result aggregation

## Future Enhancements

Potential improvements:
- Parallel execution for independent tools
- Dependency resolution between batch calls
- Streaming results as they complete
- Configurable truncation limits
- Batch size limits and warnings
