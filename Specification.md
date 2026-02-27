# vibebot - Technical Specification for AI Coding Agents

**Version**: 1.0  
**Last Updated**: 2026-02-27  
**Status**: Production Ready

## Project Overview

vibebot is a production-ready AI assistant bot written in Go with advanced features including project management, dynamic model switching, and automatic context consolidation.

**Language**: Go 1.24  
**Primary Gateway**: Telegram  
**LLM Provider**: OpenRouter (supports multiple models)

## Current Implementation Status

### ✅ Completed Features

**Core Agent System**
- Agent loop with iterative tool execution (max 40 iterations)
- Command handling (/new, /help, /model, /project)
- Error handling and graceful degradation
- Max history messages: 50 in context

**Memory System (3-layer)**
- GlobalMemory.md - Cross-project facts (always loaded)
- Project Memory - Per-project context (loaded when active)
- HISTORY.md - Append-only conversation log

**Session Management**
- Per-user session tracking (telegram_<chatID>.json)
- Message history storage
- Current project tracking
- Consolidation state (LastConsolidated field)
- Session persistence to disk

**Project Management**
- Create projects with template
- Switch between projects
- List all projects
- Delete projects
- Clear current project
- Project name validation (alphanumeric, hyphens, underscores, 1-64 chars)

**Model Management**
- Dynamic model switching at runtime
- Model persistence across restarts (current_model.txt)
- Model validation against allowed list
- Thread-safe operations (sync.RWMutex)

**Automatic Consolidation**
- Triggers at 100 messages threshold
- Batch size: 50 messages
- LLM-based summarization
- Fact extraction (global vs project)
- Background processing (non-blocking)
- Updates GlobalMemory.md and project memory
- Appends summary to HISTORY.md

**Tools**
- `read_file` - Read file contents (workspace-relative)
- `write_file` - Write/create files with directory creation
- `edit_file` - Find and replace in files
- `list_dir` - List directory with sizes
- `exec` - Shell command execution (60s timeout, 10K output limit)
- `message` - Send messages (context-aware chat ID)

**Safety Features**
- User whitelisting (TELEGRAM_ALLOWED_USERS)
- Dangerous command blocking (rm -rf, format, shutdown, etc.)
- Command timeout (60 seconds)
- Output truncation (10K characters)
- Workspace-relative path resolution
- Max iteration limit (40)

### 📋 Planned Features

- Web search tool (DuckDuckGo)
- Web fetch tool (URL content extraction)
- Cron/scheduling system
- Self-modification capabilities
- Discord gateway
- Vector embeddings for semantic search

## Architecture

### Directory Structure

```
vibebot/
├── cmd/vibebot/main.go           # Entry point
├── internal/
│   ├── agent/
│   │   ├── agent.go              # Core agent loop
│   │   └── system_prompt.go      # System prompt
│   ├── consolidation/
│   │   └── consolidation.go      # Auto consolidation
│   ├── memory/
│   │   └── memory.go             # Memory system
│   ├── session/
│   │   └── session.go            # Session management
│   ├── modelmanager/
│   │   └── modelmanager.go       # Model switching
│   ├── llm/
│   │   ├── provider.go           # LLM interface
│   │   └── openrouter.go         # OpenRouter impl
│   ├── tools/
│   │   ├── registry.go           # Tool registry
│   │   ├── file.go               # File tools
│   │   ├── exec.go               # Shell execution
│   │   └── message.go            # Messaging
│   ├── config/
│   │   └── config.go             # Configuration
│   └── logger/
│       └── logger.go             # Logging
├── pkg/telegram/
│   └── gateway.go                # Telegram bot
└── workspace/                    # Runtime data
    ├── memory/
    │   ├── GlobalMemory.md       # Global facts
    │   ├── HISTORY.md            # Conversation log
    │   └── current_model.txt     # Model state
    ├── projects/
    │   └── *.md                  # Project memories
    └── sessions/
        └── telegram_*.json       # User sessions
```

### Key Components

**Agent (`internal/agent/agent.go`)**
- ProcessMessage() - Main entry point
- Handles commands directly (no LLM for /project, /model, etc.)
- Loads global + project memory
- Iterative tool execution loop
- Session persistence
- Consolidation check (background)

**Memory (`internal/memory/memory.go`)**
- LoadMemory() / SaveMemory() - Global memory
- LoadProjectMemory() / SaveProjectMemory() - Project memory
- CreateProject() / DeleteProject() / ListProjects()
- LogConversation() - Append to HISTORY.md
- AppendGlobalFacts() / AppendProjectFacts() - Consolidation
- AppendConsolidationSummary() - Summary to HISTORY.md
- ValidateProjectName() - Security validation

**Session (`internal/session/session.go`)**
- Session struct: Key, Messages, CurrentProject, LastConsolidated
- GetOrCreate() - Get or create session
- Save() - Persist to disk (JSON)
- AddMessage() - Add to conversation
- GetHistory() - Get recent messages (limit)
- SetProject() / GetProject() / ClearProject()
- NeedsConsolidation() - Check threshold
- GetMessagesForConsolidation() - Get batch
- MarkConsolidated() - Update state

**ModelManager (`internal/modelmanager/modelmanager.go`)**
- GetCurrent() - Get current model
- SetCurrent() - Set and persist model
- GetAllowed() - List allowed models
- saveModel() / loadSavedModel() - Persistence

**Consolidator (`internal/consolidation/consolidation.go`)**
- ConsolidateMessages() - Main consolidation
- buildConsolidationPrompt() - Create prompt
- parseConsolidationResponse() - Parse LLM output
- extractFacts() - Extract bullet points

**Tools (`internal/tools/`)**
- Registry - Tool registration and execution
- Handler type: func(ctx, args) (string, error)
- Execute() - Parse JSON args and call handler

### Data Flow

**Message Processing**:
1. Telegram gateway receives message
2. Agent.ProcessMessage(chatID, message)
3. Handle commands (/new, /help, /model, /project) → return early
4. Get/create session
5. Add user message to session
6. Load GlobalMemory.md
7. Load project memory (if CurrentProject set)
8. Build system message (prompt + global + project)
9. Build messages array (system + history)
10. Agent loop (max 40 iterations):
    - Call LLM with tools
    - If tool calls: execute, add results, continue
    - If no tool calls: break with response
11. Add assistant response to session
12. Save session to disk
13. Log conversation to HISTORY.md
14. Check consolidation (background if needed)
15. Return response to gateway

**Consolidation Flow** (background):
1. Check session.NeedsConsolidation(100)
2. Get messages for consolidation (batch of 50)
3. Build consolidation prompt
4. Call LLM (no tools)
5. Parse response (summary + global facts + project facts)
6. Append global facts to GlobalMemory.md
7. Append project facts to project memory (if active)
8. Append summary to HISTORY.md
9. Update session.LastConsolidated
10. Save session

**Project Switch**:
1. User sends `/project myproject`
2. Agent validates project name
3. Check project exists (memory.ProjectExists)
4. Load project memory (verify readable)
5. session.SetProject(name)
6. Save session
7. Return confirmation

**Model Switch**:
1. User sends `/model gpt-4`
2. Agent parses command
3. Find matching model in allowed list
4. modelManager.SetCurrent(model)
5. Persist to current_model.txt
6. Return confirmation

## Configuration

**Environment Variables** (.env):
```env
# Telegram
TELEGRAM_TOKEN=<bot_token>
TELEGRAM_ALLOWED_USERS=<comma_separated_user_ids>

# LLM
OPENROUTER_API_KEY=<api_key>
OPENROUTER_MODEL=<default_model>
OPENROUTER_ALLOWED_MODELS=<comma_separated_models>

# Storage
WORKSPACE_DIR=<path>

# Logging
LOG_LEVEL=info  # debug, info, warn, error, fatal
```

**Constants** (`internal/agent/agent.go`):
```go
maxToolIterations = 40
maxHistoryMessages = 50
consolidationThreshold = 100
consolidationBatchSize = 50
```

## API Interfaces

**LLM Provider** (`internal/llm/provider.go`):
```go
type Provider interface {
    Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error)
    Name() string
}

type Message struct {
    Role       string      // system, user, assistant, tool
    Content    string
    ToolCalls  []ToolCall  // For assistant messages
    ToolCallID string      // For tool messages
}

type Response struct {
    Content      string
    ToolCalls    []ToolCall
    FinishReason string
}
```

**Tool Handler** (`internal/tools/registry.go`):
```go
type Handler func(ctx context.Context, args map[string]interface{}) (string, error)

type Tool struct {
    Definition llm.Tool
    Handler    Handler
}
```

## File Formats

**Session** (sessions/telegram_<chatID>.json):
```json
{
  "Key": "telegram:123456",
  "Messages": [...],
  "CurrentProject": "myproject",
  "LastConsolidated": 50,
  "CreatedAt": "2026-02-27T...",
  "UpdatedAt": "2026-02-27T..."
}
```

**Model State** (memory/current_model.txt):
```
openai/gpt-4-turbo
```

**Project Memory** (projects/myproject.md):
```markdown
# Project: myproject

## Description
[Brief description]

## Status
Active

## Key Facts
- Created: 2026-02-27
- ...

## Current Focus
[What you're working on]

## Notes
[Details]
```

## Security Considerations

- User whitelisting enforced at gateway level
- Project name validation prevents path traversal
- Workspace-relative paths only
- Dangerous commands blocked
- Command timeouts prevent hanging
- Output truncation prevents memory issues
- Model validation against allowed list
- Session files use safe permissions (0644)

## Performance Characteristics

- **Startup time**: <1 second
- **Message latency**: 2-10 seconds (LLM dependent)
- **Memory usage**: ~50MB base + LLM context
- **Consolidation overhead**: ~5-10 seconds (background)
- **Session file size**: ~1-10KB per user
- **Model state file**: <100 bytes

## Known Limitations

- Single-threaded agent loop (sequential message processing)
- No rate limiting (could hit API limits)
- Tool execution is synchronous (no parallelization)
- Consolidation errors logged but not surfaced to user
- No automatic cleanup of old session files
- Shared workspace (all users access same files)

## Development Guidelines

**Adding New Tools**:
1. Create handler function in `internal/tools/`
2. Define tool schema (name, description, parameters)
3. Register in tool registry
4. Add safety checks if needed

**Adding New Commands**:
1. Add handler in `internal/agent/agent.go`
2. Check command prefix in ProcessMessage()
3. Return response directly (no LLM needed)
4. Update help text

**Modifying Memory System**:
1. Update `internal/memory/memory.go`
2. Consider migration for existing data
3. Update documentation
4. Test with existing workspace

**Adding LLM Providers**:
1. Implement `llm.Provider` interface
2. Add configuration options
3. Update initialization in `cmd/vibebot/main.go`

## Testing Strategy

**Manual Testing**:
- Basic conversation flow
- Tool execution (file ops, shell, message)
- Project management (create, switch, delete)
- Model switching
- Session persistence (restart bot)
- Consolidation (reach 100 messages)

**Integration Points to Verify**:
- Telegram gateway → Agent
- Agent → LLM provider
- Agent → Tools
- Agent → Memory system
- Agent → Session manager
- Consolidator → Memory system

## Deployment

**Build**:
```bash
go build -o vibebot cmd/vibebot/main.go
```

**Run**:
```bash
export $(grep -v '^#' .env | xargs)
./vibebot gateway
```

**Systemd** (production):
```bash
sudo cp vibebot.service /etc/systemd/system/
sudo systemctl enable vibebot
sudo systemctl start vibebot
```

**Logs**:
```bash
journalctl -u vibebot -f
```

## Future Enhancements

**High Priority**:
- Web search tool
- Web fetch tool
- Cron/scheduling system

**Medium Priority**:
- Self-modification capabilities
- Additional LLM providers (Anthropic direct, OpenAI)
- Discord gateway
- CLI gateway
- Metrics and observability

**Low Priority**:
- Vector embeddings for semantic search
- Automatic memory consolidation improvements
- Plugin system for third-party tools
- Web UI for configuration
- Per-user workspaces

---

**For AI Coding Agents**: This specification provides the complete technical context for understanding and modifying vibebot. All features marked as ✅ are fully implemented and tested. Use this as the source of truth for the current state of the project.
