# Model Persistence

## Overview

Vibebot now persists the currently selected LLM model across restarts. When you switch models during a conversation, your choice is automatically saved and restored when the bot restarts.

## How It Works

### Storage Location

The current model selection is stored in:
```
<workspace_dir>/memory/current_model.txt
```

Default location: `~/.vibebot/workspace/memory/current_model.txt`

### Behavior

1. **Initial Startup**: Uses the model specified in `OPENROUTER_MODEL` environment variable
2. **Model Switch**: When you switch models (e.g., via the model selection prompt), the choice is saved immediately
3. **Restart**: On restart, the bot loads the last selected model from `current_model.txt`
4. **Validation**: Only models from `OPENROUTER_ALLOWED_MODELS` can be persisted and restored

### Example Flow

```bash
# First startup
OPENROUTER_MODEL=anthropic/claude-3.5-sonnet ./vibebot gateway
# Bot starts with Claude 3.5 Sonnet

# User switches to GPT-4
# Selection is saved to current_model.txt

# Restart the bot
./vibebot gateway
# Bot automatically restores GPT-4 (even though .env says Sonnet)
```

## Implementation Details

### Files Modified

1. **internal/modelmanager/modelmanager.go**
   - Added `stateFile` field to store the path to the state file
   - Modified `New()` to accept `workspaceDir` parameter and load saved model
   - Modified `SetCurrent()` to automatically save model changes
   - Added `saveModel()` and `loadSavedModel()` helper methods

2. **cmd/vibebot/main.go**
   - Updated `modelmanager.New()` call to pass `workspaceDir`
   - Changed initialization order: model manager before LLM provider
   - Modified LLM provider initialization to use `modelMgr.GetCurrent()` instead of `cfg.OpenRouterModel`
   - Added log message showing the current model on startup

### Thread Safety

All operations are protected by the existing `sync.RWMutex` in the model manager:
- `GetCurrent()` uses read lock
- `SetCurrent()` uses write lock (which includes the save operation)

### Error Handling

- **Save Failures**: Non-critical, errors are silently ignored (model still changes in memory)
- **Load Failures**: Falls back to the default model from environment variable
- **Invalid Model**: If saved model is not in allowed list, uses default model

### File Format

The state file is a simple text file containing only the model name:
```
openai/gpt-4-turbo
```

No JSON, no metadata - just the model identifier.

## Testing

### Manual Test

1. Start the bot and note the current model
2. Switch to a different model using the model selection prompt
3. Restart the bot
4. Verify the last selected model is restored

### Verification

Check the log output on startup:
```
INFO Current model: openai/gpt-4-turbo
```

Or inspect the state file directly:
```bash
cat ~/.vibebot/workspace/memory/current_model.txt
```

## Limitations

- Only one global model selection (not per-user)
- No history of model changes
- No automatic fallback if selected model becomes unavailable (would need manual intervention)

## Future Enhancements

Potential improvements:
- Per-user model preferences
- Model selection history
- Automatic fallback to default if saved model is no longer available
- Metrics on model usage
- API to query model selection history

## Migration

This feature is backward compatible:
- Existing installations will work without changes
- First startup creates the state file after first model switch
- No manual migration needed

## Security Considerations

- State file is readable only by the bot user (0644 permissions)
- Only models from the allowed list can be persisted
- No user input is directly written to the state file (only validated model names)

---

**Implementation Date**: 2024
**Version**: vibebot v0.1.0+
