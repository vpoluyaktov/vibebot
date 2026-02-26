# Model Persistence Flow Diagram

## Startup Flow

```
┌─────────────────────────────────────────────────────────────┐
│ Bot Startup                                                 │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Load Config (.env)                                       │
│    - OPENROUTER_MODEL = "claude-3.5-sonnet"                 │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Create Model Manager                                     │
│    modelmanager.New(defaultModel, allowedModels, workspace) │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Load Saved Model (if exists)                             │
│    Read: workspace/memory/current_model.txt                 │
└─────────────────────────────────────────────────────────────┘
                           │
                  ┌────────┴────────┐
                  │                 │
         File Exists?        File Missing?
                  │                 │
                  ▼                 ▼
        ┌─────────────────┐  ┌─────────────────┐
        │ Validate Model  │  │ Use Default     │
        │ Is Allowed?     │  │ from .env       │
        └─────────────────┘  └─────────────────┘
                  │                 │
         ┌────────┴────────┐        │
         │                 │        │
    Valid Model?     Not Allowed?   │
         │                 │        │
         ▼                 ▼        │
    ┌─────────┐      ┌─────────┐   │
    │ Use     │      │ Use     │   │
    │ Saved   │      │ Default │   │
    └─────────┘      └─────────┘   │
         │                 │        │
         └────────┬────────┘        │
                  │◄────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Initialize LLM Provider                                  │
│    llm.NewOpenRouter(apiKey, modelMgr.GetCurrent())         │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. Bot Ready                                                │
│    LOG: "Current model: gpt-4-turbo" (or whatever was used) │
└─────────────────────────────────────────────────────────────┘
```

## Model Switch Flow

```
┌─────────────────────────────────────────────────────────────┐
│ User Action: "Switch to GPT-4"                              │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ Agent Processes Request                                     │
│ - Shows model selection menu                                │
│ - User selects: "openai/gpt-4-turbo"                        │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ modelManager.SetCurrent("openai/gpt-4-turbo")               │
└─────────────────────────────────────────────────────────────┘
                           │
                  ┌────────┴────────┐
                  │                 │
                  ▼                 ▼
        ┌─────────────────┐  ┌─────────────────┐
        │ 1. Validate     │  │ 2. Update       │
        │    Model is     │  │    Memory       │
        │    Allowed      │  │    (in-memory)  │
        └─────────────────┘  └─────────────────┘
                  │                 │
                  └────────┬────────┘
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Persist to Disk                                          │
│    saveModel("openai/gpt-4-turbo")                          │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ Write to: workspace/memory/current_model.txt                │
│ Content: "openai/gpt-4-turbo"                               │
└─────────────────────────────────────────────────────────────┘
                           │
                  ┌────────┴────────┐
                  │                 │
           Save Success?      Save Failed?
                  │                 │
                  ▼                 ▼
        ┌─────────────────┐  ┌─────────────────┐
        │ ✅ Model        │  │ ⚠️  Model       │
        │    Changed &    │  │    Changed but  │
        │    Saved        │  │    Not Saved    │
        └─────────────────┘  └─────────────────┘
                  │                 │
                  └────────┬────────┘
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Update LLM Provider                                      │
│    provider.SetModel("openai/gpt-4-turbo")                  │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. Confirm to User                                          │
│    "✅ Switched to openai/gpt-4-turbo"                      │
└─────────────────────────────────────────────────────────────┘
```

## File Structure Timeline

### First Startup (No Saved State)
```
~/.vibebot/workspace/
└── memory/
    ├── MEMORY.md          ✓ Created by memory system
    └── HISTORY.md         ✓ Created by memory system
    
# current_model.txt doesn't exist yet
# Uses OPENROUTER_MODEL from .env
```

### After First Model Switch
```
~/.vibebot/workspace/
└── memory/
    ├── MEMORY.md          ✓ Exists
    ├── HISTORY.md         ✓ Exists
    └── current_model.txt  ✓ Created (contains: "openai/gpt-4-turbo")
```

### On Restart
```
~/.vibebot/workspace/
└── memory/
    ├── MEMORY.md          ✓ Loaded into context
    ├── HISTORY.md         ✓ Available for grep search
    └── current_model.txt  ✓ Loaded by model manager
                              Bot starts with saved model!
```

## Error Scenarios

### Scenario 1: State File Corrupted
```
Load saved model → Read file → Invalid content
                                      │
                                      ▼
                              Fall back to default
                              (.env OPENROUTER_MODEL)
```

### Scenario 2: Saved Model Not in Allowed List
```
Load saved model → Read: "some-old-model" → Validate
                                                  │
                                         Not in allowed list
                                                  │
                                                  ▼
                                    Fall back to default
                                    (.env OPENROUTER_MODEL)
```

### Scenario 3: Cannot Write State File
```
SetCurrent() → Validate → Update memory → Save to disk
                                               │
                                          Write fails
                                               │
                                               ▼
                                    Log warning (silent)
                                    Continue (model changed in memory)
                                    Next restart: uses old saved model or default
```

## Thread Safety

```
User 1: SetCurrent()          User 2: GetCurrent()
         │                              │
         ▼                              ▼
    Lock (Write)                  Lock (Read)
         │                              │
         │◄─────── Mutex ──────────────►│
         │                              │
    Update model                   Return model
    Save to disk                        │
         │                              ▼
    Unlock                          Unlock
```

All operations are protected by `sync.RWMutex` ensuring:
- Multiple readers can read simultaneously
- Writers get exclusive access
- No race conditions
- Thread-safe model persistence

---

**Visual Guide Version**: 1.0  
**Last Updated**: 2024-02-26
