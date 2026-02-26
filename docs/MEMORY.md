# Memory System

vibebot uses a two-layer memory system inspired by nanobot's architecture.

## Architecture

```
workspace/
└── memory/
    ├── MEMORY.md   # Long-term facts (always loaded)
    └── HISTORY.md  # Conversation log (searchable)
```

## Layer 1: MEMORY.md (Long-term Memory)

**Purpose**: Store important facts that should persist across sessions.

**Characteristics**:
- Always loaded into the LLM context
- Manually curated by the bot
- Contains structured information
- Should be kept concise and relevant

**What to store**:
- User preferences and information
- Important facts and relationships
- Active project context
- System configuration
- Recurring tasks or reminders

**How to update**:
```go
// Read current memory
content := read_file("memory/MEMORY.md")

// Edit specific section
edit_file("memory/MEMORY.md", "old text", "new text")

// Or rewrite entirely
write_file("memory/MEMORY.md", newContent)
```

**Example structure**:
```markdown
# Long-term Memory

## User Information
- Name: John Doe
- Timezone: UTC-8
- Preferences: Concise responses, technical details

## Active Projects
- Project A: Working on authentication system
- Project B: Debugging performance issues

## Important Facts
- Deployment happens on Fridays
- Use staging environment for testing
```

## Layer 2: HISTORY.md (Conversation Log)

**Purpose**: Append-only log of all conversations.

**Characteristics**:
- NOT loaded into context (too large)
- Automatically populated after each conversation
- Searchable using grep
- Never edited, only appended

**Format**:
```markdown
---
[2026-02-26 15:24:00 UTC] Chat: 339899302

User: What's the weather like?

Bot: I don't have a weather tool yet, but I can help you...

---
[2026-02-26 15:25:00 UTC] Chat: 339899302

User: Can you read that file?

Bot: I'll read the file for you...
```

**How to search**:
```bash
# Search for keyword
exec("grep -i 'weather' memory/HISTORY.md")

# Search for multiple keywords
exec("grep -iE 'weather|temperature' memory/HISTORY.md")

# Search with context (3 lines before/after)
exec("grep -i -C 3 'weather' memory/HISTORY.md")

# Search for recent conversations
exec("tail -n 100 memory/HISTORY.md | grep -i 'keyword'")
```

## Best Practices

### For the Bot

1. **Update MEMORY.md proactively**
   - When user shares preferences
   - When starting new projects
   - When important facts emerge

2. **Keep MEMORY.md concise**
   - Remove outdated information
   - Consolidate related facts
   - Use clear structure

3. **Search HISTORY.md when needed**
   - User asks "what did I say about X?"
   - Need to recall past conversations
   - Looking for specific information

4. **Log important decisions**
   - Add to MEMORY.md if it affects future behavior
   - Let HISTORY.md capture the full conversation

### For Developers

1. **Initialize memory on first run**
   - Template is created automatically
   - Customize for your use case

2. **Monitor memory size**
   - MEMORY.md should stay under 10KB
   - HISTORY.md can grow indefinitely

3. **Backup regularly**
   - Both files are plain text
   - Easy to version control
   - Can be synced across instances

## Implementation Details

### Memory Manager

```go
type Memory struct {
    workspaceDir string
    memoryFile   string  // MEMORY.md path
    historyFile  string  // HISTORY.md path
}

// Load long-term memory (called on every request)
func (m *Memory) LoadMemory() (string, error)

// Save long-term memory (called by bot when updating)
func (m *Memory) SaveMemory(content string) error

// Log conversation (called automatically after each response)
func (m *Memory) LogConversation(chatID int64, userMsg, botResp string) error
```

### Automatic Logging

Every conversation is automatically logged to HISTORY.md:

```go
// In agent.ProcessMessage()
defer func() {
    if err := a.memory.LogConversation(chatID, message, finalResponse); err != nil {
        log.Printf("Warning: failed to log conversation: %v", err)
    }
}()
```

### System Prompt Integration

The system prompt explains the memory system to the LLM:

```go
systemMessage := systemPrompt
if memoryContent != "" {
    systemMessage += "\n\n## Current Memory (MEMORY.md)\n\n" + memoryContent
}
```

## Future Enhancements

- [ ] Automatic memory consolidation (summarize old HISTORY.md entries)
- [ ] Memory search tool (dedicated tool instead of grep)
- [ ] Memory statistics (size, age, usage)
- [ ] Multi-user memory isolation
- [ ] Memory export/import
- [ ] Semantic search over history
