# Architecture

vibebot is a modern AI assistant built with clean architecture principles in Go.

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         User (Telegram)                      │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    Telegram Gateway                          │
│  - Receives messages                                         │
│  - Sends responses                                           │
│  - Handles bot API                                           │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                         Agent                                │
│  - Core decision loop                                        │
│  - Manages conversation flow                                │
│  - Coordinates LLM and tools                                 │
│  - Handles memory                                            │
└─────┬──────────────┬──────────────┬────────────────────────┘
      │              │              │
      ▼              ▼              ▼
┌──────────┐  ┌──────────┐  ┌──────────────┐
│   LLM    │  │  Tools   │  │    Memory    │
│ Provider │  │ Registry │  │   System     │
└──────────┘  └──────────┘  └──────────────┘
```

## Components

### 1. Telegram Gateway (`pkg/telegram`)

**Responsibility**: Handle Telegram Bot API communication

**Key Features**:
- Receives updates from Telegram
- Routes messages to agent
- Sends responses back to users
- Handles message formatting (Markdown)

**Interface**:
```go
type Gateway struct {
    bot     *tgbotapi.BotAPI
    handler MessageHandler
}

func New(token string, handler MessageHandler) (*Gateway, error)
func (g *Gateway) Start(ctx context.Context) error
func (g *Gateway) SendMessage(chatID int64, text string) error
```

### 2. Agent (`internal/agent`)

**Responsibility**: Core AI decision-making and orchestration

**Key Features**:
- Iterative tool execution loop
- Context management
- Memory integration
- Error handling

**Flow**:
1. Receive user message
2. Load memory context
3. Build LLM messages with system prompt
4. Call LLM with available tools
5. If tool calls requested:
   - Execute each tool
   - Add results to conversation
   - Loop back to step 4
6. Return final response
7. Log conversation to history

**Configuration**:
- `maxToolIterations = 10` - Prevents infinite loops

### 3. LLM Provider (`internal/llm`)

**Responsibility**: Abstract LLM API interactions

**Interface**:
```go
type Provider interface {
    Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error)
    Name() string
}
```

**Implementations**:
- `OpenRouter` - Supports any model via OpenRouter API

**Message Types**:
- `system` - System instructions and context
- `user` - User messages
- `assistant` - Bot responses (may include tool calls)
- `tool` - Tool execution results

### 4. Tools Registry (`internal/tools`)

**Responsibility**: Manage and execute available tools

**Architecture**:
```go
type Tool struct {
    Definition llm.Tool  // LLM-facing definition
    Handler    Handler   // Actual implementation
}

type Registry struct {
    tools map[string]*Tool
}
```

**Available Tools**:

| Tool | Description | Safety |
|------|-------------|--------|
| `read_file` | Read file contents | Workspace-relative paths |
| `write_file` | Write/create files | Creates directories |
| `edit_file` | Find and replace | Exact match required |
| `list_dir` | List directory | Shows sizes |
| `exec` | Run shell commands | Timeout, dangerous command blocking |
| `message` | Send messages | Context-aware chat ID |

**Tool Execution Flow**:
1. LLM requests tool call with JSON arguments
2. Registry parses arguments
3. Handler executes with context
4. Result returned as string
5. Result added to conversation

### 5. Memory System (`internal/memory`)

**Responsibility**: Persistent storage of facts and conversations

**Two-Layer Architecture**:

**MEMORY.md** (Long-term facts):
- Always loaded into LLM context
- Manually curated by bot
- Stores user preferences, project context
- Should stay concise (<10KB)

**HISTORY.md** (Conversation log):
- Append-only log
- NOT loaded into context
- Searchable via grep
- Unlimited growth

**Interface**:
```go
type Memory struct {
    workspaceDir string
    memoryFile   string
    historyFile  string
}

func (m *Memory) LoadMemory() (string, error)
func (m *Memory) SaveMemory(content string) error
func (m *Memory) LogConversation(chatID int64, userMsg, botResp string) error
```

### 6. Configuration (`internal/config`)

**Responsibility**: Environment-based configuration

**Settings**:
- `TELEGRAM_TOKEN` - Bot authentication
- `OPENROUTER_API_KEY` - LLM API key
- `OPENROUTER_MODEL` - Model selection
- `WORKSPACE_DIR` - Data storage location

**Defaults**:
- Model: `anthropic/claude-3.5-sonnet`
- Workspace: `~/.vibebot/workspace`

## Data Flow

### Incoming Message

```
User sends message
    ↓
Telegram Gateway receives update
    ↓
Gateway calls MessageHandler
    ↓
Agent.ProcessMessage(ctx, chatID, message)
    ↓
Load MEMORY.md
    ↓
Build system prompt + memory context
    ↓
┌─────────────────────────────────┐
│   Agent Loop (max 10 iterations) │
│                                  │
│  1. Call LLM with tools          │
│  2. If tool calls:               │
│     - Execute each tool          │
│     - Add results to messages    │
│     - Continue loop              │
│  3. If no tool calls:            │
│     - Break with final response  │
└─────────────────────────────────┘
    ↓
Log conversation to HISTORY.md
    ↓
Return response to Gateway
    ↓
Gateway sends to Telegram
    ↓
User receives response
```

### Tool Execution

```
LLM returns tool_calls in response
    ↓
For each tool call:
    ↓
Registry.Execute(ctx, name, argsJSON)
    ↓
Parse JSON arguments
    ↓
Get tool handler
    ↓
Execute handler(ctx, args)
    ↓
Return result string
    ↓
Add as "tool" message with tool_call_id
    ↓
Continue agent loop
```

## Safety & Reliability

### Command Execution Safety

**Blocked Commands**:
- `rm -rf` - Recursive deletion
- `mkfs`, `format` - Filesystem formatting
- `dd if=` - Low-level disk operations
- `shutdown`, `reboot`, `halt` - System control
- Fork bombs and similar

**Limits**:
- 60-second timeout per command
- 10,000 character output limit
- Workspace-relative path resolution

### Error Handling

**Graceful Degradation**:
- Memory load failures → Continue with empty memory
- Tool execution errors → Return error to LLM
- LLM API errors → Return error to user
- Max iterations → Return error message

**Logging**:
- All tool executions logged
- Errors logged with context
- Conversation logging failures are non-fatal

### Context Management

**Chat ID Propagation**:
```go
ctx = context.WithValue(ctx, "chat_id", chatID)
```

**Timeout Handling**:
- Gateway context cancellation
- Command execution timeouts
- Graceful shutdown on SIGTERM

## Extensibility

### Adding New Tools

```go
// 1. Create tool definition
registry.Register("my_tool", &Tool{
    Definition: llm.Tool{
        Type: "function",
        Function: llm.Function{
            Name: "my_tool",
            Description: "What it does",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "param": map[string]interface{}{
                        "type": "string",
                        "description": "Parameter description",
                    },
                },
                "required": []string{"param"},
            },
        },
    },
    Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
        // Implementation
        return "result", nil
    },
})
```

### Adding New LLM Providers

```go
// 1. Implement Provider interface
type MyProvider struct {
    apiKey string
}

func (p *MyProvider) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
    // Call your LLM API
}

func (p *MyProvider) Name() string {
    return "my-provider"
}

// 2. Use in main.go
provider := NewMyProvider(apiKey)
agent := agent.New(provider, mem, tools)
```

### Adding New Gateways

```go
// 1. Implement MessageHandler interface
type MyGateway struct {
    handler telegram.MessageHandler
}

func (g *MyGateway) Start(ctx context.Context) error {
    // Listen for messages
    // Call handler(ctx, chatID, message)
    // Send response
}

// 2. Register in main.go
gateway := NewMyGateway(handler)
gateway.Start(ctx)
```

## Performance Considerations

### Memory Usage

- MEMORY.md loaded on every request (~10KB)
- HISTORY.md never loaded (append-only)
- Tool definitions sent to LLM (~5KB)
- Conversation context grows with iterations

### Latency

- LLM API call: 1-5 seconds
- Tool execution: <1 second (except exec)
- File operations: <100ms
- Total per message: 2-10 seconds (depending on tool iterations)

### Scalability

**Current Limitations**:
- Single-threaded agent loop
- No request queuing
- No rate limiting
- No multi-user session isolation

**Future Improvements**:
- Per-user agent instances
- Request queue with priority
- Rate limiting per user
- Concurrent tool execution

## Security

### Current Model

- Bot token authentication only
- No user authentication
- All users share same workspace
- Commands run as bot process user

### Recommendations for Production

1. **User Authentication**
   - Whitelist allowed Telegram user IDs
   - Per-user workspaces
   - Role-based access control

2. **Command Sandboxing**
   - Run exec in container
   - Restrict filesystem access
   - Network isolation

3. **Rate Limiting**
   - Per-user message limits
   - Tool execution quotas
   - LLM API cost tracking

4. **Audit Logging**
   - All tool executions
   - File modifications
   - Command executions
   - API calls

## Testing Strategy

### Unit Tests

- Tool handlers (mock context)
- Memory operations (temp directories)
- LLM provider (mock HTTP)
- Message parsing

### Integration Tests

- Agent loop with mock LLM
- Tool execution end-to-end
- Memory persistence
- Error handling

### Manual Testing

- Telegram bot interaction
- Tool combinations
- Error scenarios
- Memory updates

## Future Architecture

### Planned Enhancements

1. **Multi-Gateway Support**
   - Discord, Slack, CLI
   - Unified message routing
   - Gateway-specific features

2. **Plugin System**
   - Dynamic tool loading
   - Third-party tools
   - Tool marketplace

3. **Advanced Memory**
   - Semantic search
   - Automatic consolidation
   - Vector embeddings
   - Multi-user isolation

4. **Self-Modification**
   - Code generation tools
   - Safe code execution
   - Version control integration
   - Rollback capabilities

5. **Observability**
   - Metrics (Prometheus)
   - Tracing (OpenTelemetry)
   - Structured logging
   - Health checks
