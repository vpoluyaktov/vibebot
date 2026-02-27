# Vibebot Project Content Switch - Implementation Specification

## Overview

This specification describes the implementation of a project-based memory system for vibebot, allowing users to switch between different project contexts while maintaining a global memory layer.

## Current State

After clearing the workspace, vibebot has:
- **No current project** - Fresh installation with empty workspace
- **Two-layer memory system**: GlobalMemory.md (long-term facts) and HISTORY.md (event log)
- **Partial project support**: Basic command handlers exist but project memory integration is incomplete

## Goals

1. Enable users to work on multiple projects with isolated context
2. Maintain global memory for cross-project information
3. Provide seamless project switching without losing context
4. Ensure project-specific facts don't pollute global memory

## Architecture

### Memory Hierarchy

```
workspace/
├── memory/
│   ├── GlobalMemory.md      # Cross-project facts (always loaded)
│   └── HISTORY.md            # Append-only event log
├── projects/
│   ├── project1.md           # Project-specific memory
│   ├── project2.md           # Project-specific memory
│   └── ...
└── sessions/
    └── telegram_<chatID>.json # Per-user conversation history + current project
```

### Context Loading Priority

When processing a message:
1. **GlobalMemory.md** - Always loaded (system-wide facts)
2. **Current Project Memory** - Loaded if a project is active (project-specific facts)
3. **Session History** - Recent conversation context

## Implementation Plan

### 1. Memory System Enhancement

**File**: `internal/memory/memory.go`

**Changes**:
- Add `projectsDir` field to Memory struct
- Add `GetProjectPath(projectName string) string` method
- Add `LoadProjectMemory(projectName string) (string, error)` method
- Add `SaveProjectMemory(projectName, content string) error` method
- Add `ListProjects() ([]string, error)` method
- Add `DeleteProject(projectName string) error` method
- Create projects directory on initialization

**Example**:
```go
type Memory struct {
    workspaceDir string
    memoryFile   string
    historyFile  string
    projectsDir  string  // NEW
}

func (m *Memory) LoadProjectMemory(projectName string) (string, error) {
    projectPath := filepath.Join(m.projectsDir, projectName+".md")
    // ... load and return content
}
```

### 2. Session Enhancement

**File**: `internal/session/session.go`

**Changes**:
- Add `CurrentProject string` field to Session struct
- Add `SetProject(projectName string)` method
- Add `GetProject() string` method
- Persist current project in session JSON

**Example**:
```go
type Session struct {
    Key            string
    Messages       []llm.Message
    CurrentProject string  // NEW
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

### 3. Agent Enhancement

**File**: `internal/agent/agent.go`

**Changes**:
- Modify `ProcessMessage()` to load current project memory from session
- Update system message construction to include project context
- Enhance `handleProjectCommand()` to update session's current project
- Add project memory to LLM context when available

**System Message Structure**:
```
## System Prompt
[Base system instructions]

## Global Memory (GlobalMemory.md)
[Global facts - always loaded]

## Current Project: <project_name>
[Project-specific memory - loaded if project is active]

## Conversation History
[Recent messages]
```

### 4. Project Command Implementation

**File**: `internal/agent/agent.go`

**Commands**:

#### `/projects`
- List all available projects
- Show current active project (if any)
- Display project count

**Output**:
```
📂 **Projects** (3 total)

✅ project1 (current)
   project2
   project3

Use `/project <name>` to switch
Use `/project create <name>` to create new
```

#### `/project create <name>`
- Validate project name (alphanumeric, hyphens, underscores only)
- Create project memory file with template
- Switch to new project automatically
- Update session

**Template**:
```markdown
# Project: <name>

## Description
[Brief project description]

## Status
Active

## Key Facts
- Created: <timestamp>
- 

## Notes
[Project-specific notes and context]
```

#### `/project <name>`
- Validate project exists
- Switch current session to project
- Load project memory
- Confirm switch

**Output**:
```
✅ Switched to project: <name>

Project memory loaded (X lines)
```

#### `/project delete <name>`
- Confirm project exists
- Prevent deletion of current project (require switch first)
- Delete project memory file
- Confirm deletion

**Output**:
```
✅ Deleted project: <name>
```

#### `/project clear`
- Clear current project (switch to no-project mode)
- Keep project file intact
- Update session

**Output**:
```
✅ Cleared current project
Now using only GlobalMemory
```

### 5. Project Memory File Structure

**Standard Template**:
```markdown
# Project: <name>

## Description
[One-line description of the project]

## Status
[Active | On Hold | Completed | Archived]

## Repository
[Git repository URL if applicable]

## Key Facts
- Created: <timestamp>
- Last Updated: <timestamp>
- [Other important facts]

## Current Focus
[What you're currently working on]

## Notes
[Detailed project-specific context, decisions, architecture notes, etc.]

## Links
- [Related documentation]
- [Issue trackers]
- [Deployment URLs]
```

### 6. Safety and Validation

**Project Name Validation**:
- Allow: `a-z`, `A-Z`, `0-9`, `-`, `_`
- Disallow: `/`, `\`, `..`, spaces, special characters
- Max length: 64 characters
- Prevent path traversal attacks

**Error Handling**:
- Graceful handling of missing project files
- Clear error messages for invalid operations
- Prevent deletion of non-existent projects
- Handle file I/O errors

**Example Validation**:
```go
func validateProjectName(name string) error {
    if len(name) == 0 || len(name) > 64 {
        return fmt.Errorf("project name must be 1-64 characters")
    }
    
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
    if !matched {
        return fmt.Errorf("project name can only contain letters, numbers, hyphens, and underscores")
    }
    
    return nil
}
```

### 7. LLM Context Integration

**System Prompt Update** (`internal/agent/system_prompt.go`):

Add project-aware instructions:
```
## Memory System

You have access to a two-layer memory system:

1. **GlobalMemory.md** - Cross-project facts (always loaded)
   - User preferences
   - System configuration
   - General knowledge

2. **Project Memory** - Project-specific context (loaded when project is active)
   - Project-specific facts
   - Architecture decisions
   - Current focus and status

When a project is active, you have access to both global and project-specific memory.
Save project-specific facts to the project memory file, not GlobalMemory.

## Project Commands

- `/projects` - List all projects
- `/project create <name>` - Create and switch to new project
- `/project <name>` - Switch to existing project
- `/project delete <name>` - Delete a project
- `/project clear` - Clear current project (use only GlobalMemory)
```

### 8. Testing Plan

**Unit Tests**:
- [ ] Project name validation
- [ ] Project creation with valid/invalid names
- [ ] Project switching
- [ ] Project deletion
- [ ] Project memory loading/saving
- [ ] Session project persistence

**Integration Tests**:
- [ ] Create project → verify file exists
- [ ] Switch project → verify session updated
- [ ] Delete project → verify file removed
- [ ] Load project memory → verify content in LLM context
- [ ] Clear project → verify only GlobalMemory loaded

**Manual Tests**:
1. Create multiple projects
2. Switch between projects
3. Verify context isolation (facts in project1 don't appear in project2)
4. Test project deletion
5. Test project clearing
6. Verify session persistence across restarts

## Implementation Order

1. ✅ **Phase 1: Memory System** (internal/memory/memory.go) - COMPLETED (commit 1f2f71d)
   - ✅ Add projects directory support
   - ✅ Implement project CRUD operations
   - ✅ Add project listing

2. ✅ **Phase 2: Session Integration** (internal/session/session.go) - COMPLETED (commit d4caf1e)
   - ✅ Add CurrentProject field
   - ✅ Persist project in session JSON

3. ✅ **Phase 3: Agent Integration** (internal/agent/agent.go) - COMPLETED (commit 3522c3d)
   - ✅ Load project memory in ProcessMessage
   - ✅ Update system message construction
   - ✅ Enhance project command handlers

4. ✅ **Phase 4: System Prompt** (internal/agent/system_prompt.go) - COMPLETED (commit 1ed653d)
   - ✅ Add project-aware instructions
   - ✅ Document project commands

5. ⏳ **Phase 5: Testing** - IN PROGRESS
   - ⏳ Unit tests
   - ⏳ Integration tests
   - ⏳ Manual testing

6. ⏳ **Phase 6: Documentation** - IN PROGRESS
   - ⏳ Update README.md
   - ⏳ Add project workflow examples
   - ⏳ Document best practices

## Success Criteria

- ✅ Users can create, switch, delete, and list projects
- ✅ Project-specific memory is isolated from global memory
- ✅ Current project persists across sessions
- ✅ LLM receives both global and project context
- ✅ Project commands work without calling LLM
- ⏳ All tests pass (pending)
- ⏳ Documentation is complete (pending)

## Future Enhancements

- Project templates (e.g., "web-app", "research", "writing")
- Project metadata (tags, categories, priority)
- Project search and filtering
- Project archiving (move to archive/ directory)
- Project export/import
- Multi-user project sharing
- Project-specific tool configurations

## Notes

- This implementation maintains backward compatibility with existing GlobalMemory.md
- Projects are optional - users can work without projects using only GlobalMemory
- Project memory files are simple Markdown for easy manual editing
- Session-based project tracking ensures each user/chat has independent project context

---

# Automatic Context Consolidation & Memory Update - Implementation Specification

## Overview

This specification describes the implementation of automatic context consolidation and memory updates for vibebot. The system will periodically summarize old conversation history, extract important facts to memory files (GlobalMemory.md and project-specific memory), and prevent unbounded session growth.

## Current State

**What Exists**:
- ✅ `LastConsolidated` field in Session struct (currently unused, always 0)
- ✅ `GetHistory()` method that returns "unconsolidated messages"
- ✅ GlobalMemory.md and project memory files
- ✅ HISTORY.md append-only log
- ✅ Session persistence to disk

**What's Missing**:
- ❌ Automatic consolidation logic
- ❌ LLM-based summarization of old messages
- ❌ Automatic fact extraction to memory files
- ❌ Periodic consolidation triggers
- ❌ Memory update mechanisms

## Goals

1. **Prevent unbounded session growth** - Consolidate old messages to keep sessions manageable
2. **Automatic fact extraction** - Extract important information to GlobalMemory.md or project memory
3. **Preserve conversation history** - Append summaries to HISTORY.md for searchability
4. **Maintain context quality** - Keep recent messages, consolidate old ones
5. **Transparent operation** - User doesn't need to manage consolidation manually

## Architecture

### Consolidation Workflow

```
Session Messages (0...N)
│
├─ Recent Messages (LastConsolidated...N)
│  └─> Kept in full detail for LLM context
│
└─ Old Messages (0...LastConsolidated)
   ├─> Summarized by LLM
   ├─> Facts extracted to GlobalMemory.md or project memory
   ├─> Summary appended to HISTORY.md
   └─> Removed from session (or marked as consolidated)
```

### Consolidation Trigger

**When to consolidate**:
- Session exceeds threshold (e.g., 100 messages)
- Consolidate oldest 50 messages, keep recent 50
- Run after each agent response (check threshold)

**Threshold Configuration**:
```go
const (
    consolidationThreshold = 100  // Trigger when session exceeds this
    consolidationBatchSize = 50   // How many old messages to consolidate
    maxHistoryMessages     = 50   // Keep this many recent messages
)
```

## Implementation Plan

### 1. Consolidation Service

**File**: `internal/consolidation/consolidation.go` (NEW)

**Purpose**: Encapsulate consolidation logic separate from session management

**Key Components**:

```go
package consolidation

import (
    "context"
    "fmt"
    "time"
    
    "github.com/vpoluyaktov/vibebot/internal/llm"
    "github.com/vpoluyaktov/vibebot/internal/memory"
    "github.com/vpoluyaktov/vibebot/internal/logger"
)

// Consolidator handles session consolidation
type Consolidator struct {
    llmProvider llm.Provider
    memory      *memory.Memory
}

// New creates a new Consolidator
func New(llmProvider llm.Provider, mem *memory.Memory) *Consolidator {
    return &Consolidator{
        llmProvider: llmProvider,
        memory:      mem,
    }
}

// ConsolidateMessages summarizes old messages and extracts facts
func (c *Consolidator) ConsolidateMessages(
    ctx context.Context,
    messages []llm.Message,
    projectName string,
) (*ConsolidationResult, error) {
    // 1. Build consolidation prompt
    // 2. Call LLM to summarize and extract facts
    // 3. Parse response (summary + facts)
    // 4. Return result
}

// ConsolidationResult contains the output of consolidation
type ConsolidationResult struct {
    Summary      string    // Human-readable summary for HISTORY.md
    GlobalFacts  []string  // Facts to add to GlobalMemory.md
    ProjectFacts []string  // Facts to add to project memory (if project active)
    Timestamp    time.Time
}
```

**Consolidation Prompt**:
```
You are a memory consolidation assistant. Your task is to:

1. Summarize the following conversation messages into a concise summary
2. Extract important facts that should be remembered long-term
3. Categorize facts as either:
   - GLOBAL: Cross-project facts (user preferences, general knowledge)
   - PROJECT: Project-specific facts (only if project is active)

## Messages to Consolidate

[Insert old messages here]

## Current Project

[Project name or "None"]

## Output Format

Provide your response in the following format:

### SUMMARY
[2-3 paragraph summary of the conversation]

### GLOBAL_FACTS
- [Fact 1]
- [Fact 2]
...

### PROJECT_FACTS
- [Fact 1]
- [Fact 2]
...

Be concise and extract only truly important information.
```

### 2. Session Enhancement

**File**: `internal/session/session.go`

**Changes**:

```go
// Add method to check if consolidation is needed
func (s *Session) NeedsConsolidation(threshold int) bool {
    return len(s.Messages) > threshold
}

// Add method to get messages for consolidation
func (s *Session) GetMessagesForConsolidation(batchSize int) []llm.Message {
    if s.LastConsolidated >= len(s.Messages) {
        return nil
    }
    
    end := s.LastConsolidated + batchSize
    if end > len(s.Messages) {
        end = len(s.Messages)
    }
    
    return s.Messages[s.LastConsolidated:end]
}

// Add method to mark messages as consolidated
func (s *Session) MarkConsolidated(count int) {
    s.LastConsolidated += count
    s.UpdatedAt = time.Now()
}

// Optional: Add method to prune consolidated messages
func (s *Session) PruneConsolidated() {
    if s.LastConsolidated > 0 {
        s.Messages = s.Messages[s.LastConsolidated:]
        s.LastConsolidated = 0
    }
}
```

### 3. Memory Enhancement

**File**: `internal/memory/memory.go`

**Changes**:

```go
// Add method to append facts to GlobalMemory.md
func (m *Memory) AppendGlobalFacts(facts []string) error {
    if len(facts) == 0 {
        return nil
    }
    
    // Read current content
    content, err := m.LoadGlobalMemory()
    if err != nil {
        return err
    }
    
    // Find or create "## Consolidated Facts" section
    // Append facts with timestamp
    timestamp := time.Now().Format("2006-01-02 15:04")
    
    newContent := content + "\n\n## Consolidated Facts (" + timestamp + ")\n\n"
    for _, fact := range facts {
        newContent += "- " + fact + "\n"
    }
    
    return m.SaveGlobalMemory(newContent)
}

// Add method to append facts to project memory
func (m *Memory) AppendProjectFacts(projectName string, facts []string) error {
    if len(facts) == 0 {
        return nil
    }
    
    // Similar to AppendGlobalFacts but for project memory
    content, err := m.LoadProjectMemory(projectName)
    if err != nil {
        return err
    }
    
    timestamp := time.Now().Format("2006-01-02 15:04")
    newContent := content + "\n\n## Consolidated Facts (" + timestamp + ")\n\n"
    for _, fact := range facts {
        newContent += "- " + fact + "\n"
    }
    
    return m.SaveProjectMemory(projectName, newContent)
}

// Add method to append summary to HISTORY.md
func (m *Memory) AppendConsolidationSummary(summary string, projectName string) error {
    timestamp := time.Now().Format("2006-01-02 15:04:05")
    
    entry := fmt.Sprintf("\n---\n## Consolidation Summary - %s\n", timestamp)
    if projectName != "" {
        entry += fmt.Sprintf("**Project**: %s\n\n", projectName)
    }
    entry += summary + "\n"
    
    return m.AppendHistory(entry)
}
```

### 4. Agent Integration

**File**: `internal/agent/agent.go`

**Changes**:

```go
import (
    "github.com/vpoluyaktov/vibebot/internal/consolidation"
)

// Add consolidator to Agent struct
type Agent struct {
    llmProvider   llm.Provider
    toolRegistry  *tools.Registry
    memory        *memory.Memory
    sessionMgr    *session.Manager
    modelMgr      *modelmanager.ModelManager
    consolidator  *consolidation.Consolidator  // NEW
}

// Update New() to initialize consolidator
func New(llmProvider llm.Provider, mem *memory.Memory, sessionMgr *session.Manager, modelMgr *modelmanager.ModelManager) *Agent {
    return &Agent{
        llmProvider:  llmProvider,
        toolRegistry: tools.NewRegistry(),
        memory:       mem,
        sessionMgr:   sessionMgr,
        modelMgr:     modelMgr,
        consolidator: consolidation.New(llmProvider, mem),  // NEW
    }
}

// Add consolidation check after saving session
func (a *Agent) ProcessMessage(ctx context.Context, sessionKey, userMessage string) (string, error) {
    // ... existing code ...
    
    // Save session
    if err := a.sessionMgr.SaveSession(sess); err != nil {
        logger.Error("Failed to save session: %v", err)
    }
    
    // NEW: Check if consolidation is needed
    if sess.NeedsConsolidation(100) {
        go a.consolidateSession(sessionKey)  // Run in background
    }
    
    return response, nil
}

// Add consolidation method
func (a *Agent) consolidateSession(sessionKey string) {
    ctx := context.Background()
    
    sess, err := a.sessionMgr.GetSession(sessionKey)
    if err != nil {
        logger.Error("Failed to get session for consolidation: %v", err)
        return
    }
    
    // Get messages to consolidate
    messages := sess.GetMessagesForConsolidation(50)
    if len(messages) == 0 {
        return
    }
    
    logger.Info("Consolidating %d messages for session %s", len(messages), sessionKey)
    
    // Run consolidation
    result, err := a.consolidator.ConsolidateMessages(ctx, messages, sess.GetProject())
    if err != nil {
        logger.Error("Consolidation failed: %v", err)
        return
    }
    
    // Update memory files
    if len(result.GlobalFacts) > 0 {
        if err := a.memory.AppendGlobalFacts(result.GlobalFacts); err != nil {
            logger.Error("Failed to append global facts: %v", err)
        }
    }
    
    if sess.GetProject() != "" && len(result.ProjectFacts) > 0 {
        if err := a.memory.AppendProjectFacts(sess.GetProject(), result.ProjectFacts); err != nil {
            logger.Error("Failed to append project facts: %v", err)
        }
    }
    
    // Append summary to HISTORY.md
    if err := a.memory.AppendConsolidationSummary(result.Summary, sess.GetProject()); err != nil {
        logger.Error("Failed to append consolidation summary: %v", err)
    }
    
    // Mark messages as consolidated
    sess.MarkConsolidated(len(messages))
    
    // Optional: Prune consolidated messages to save space
    // sess.PruneConsolidated()
    
    // Save updated session
    if err := a.sessionMgr.SaveSession(sess); err != nil {
        logger.Error("Failed to save session after consolidation: %v", err)
    }
    
    logger.Info("Consolidation complete for session %s", sessionKey)
}
```

### 5. Configuration

**File**: `.env` or `internal/config/config.go`

**New Settings**:
```env
# Consolidation settings
CONSOLIDATION_ENABLED=true
CONSOLIDATION_THRESHOLD=100
CONSOLIDATION_BATCH_SIZE=50
CONSOLIDATION_MODEL=arcee-ai/trinity-large-preview:free  # Can use cheaper model
```

### 6. Testing Plan

**Unit Tests**:

**File**: `internal/consolidation/consolidation_test.go`
- [ ] Test consolidation prompt generation
- [ ] Test parsing of consolidation results
- [ ] Test fact categorization (global vs project)
- [ ] Test error handling

**File**: `internal/session/session_test.go`
- [ ] Test NeedsConsolidation()
- [ ] Test GetMessagesForConsolidation()
- [ ] Test MarkConsolidated()
- [ ] Test PruneConsolidated()

**File**: `internal/memory/memory_test.go`
- [ ] Test AppendGlobalFacts()
- [ ] Test AppendProjectFacts()
- [ ] Test AppendConsolidationSummary()

**Integration Tests**:
- [ ] Create session with 150 messages → verify consolidation triggers
- [ ] Verify facts appear in GlobalMemory.md
- [ ] Verify facts appear in project memory (when project active)
- [ ] Verify summary appears in HISTORY.md
- [ ] Verify LastConsolidated is updated
- [ ] Verify session size is reduced (if pruning enabled)

**Manual Tests**:
1. Have long conversation (100+ messages)
2. Verify consolidation happens automatically
3. Check GlobalMemory.md for extracted facts
4. Check HISTORY.md for summary
5. Verify session still works after consolidation
6. Test with and without active project

## Implementation Order

1. ⏳ **Phase 1: Consolidation Service** (internal/consolidation/consolidation.go)
   - [ ] Create Consolidator struct
   - [ ] Implement ConsolidateMessages()
   - [ ] Design consolidation prompt
   - [ ] Parse LLM response (summary + facts)

2. ⏳ **Phase 2: Session Enhancement** (internal/session/session.go)
   - [ ] Add NeedsConsolidation()
   - [ ] Add GetMessagesForConsolidation()
   - [ ] Add MarkConsolidated()
   - [ ] Add PruneConsolidated() (optional)

3. ⏳ **Phase 3: Memory Enhancement** (internal/memory/memory.go)
   - [ ] Add AppendGlobalFacts()
   - [ ] Add AppendProjectFacts()
   - [ ] Add AppendConsolidationSummary()

4. ⏳ **Phase 4: Agent Integration** (internal/agent/agent.go)
   - [ ] Add consolidator to Agent struct
   - [ ] Add consolidation check in ProcessMessage()
   - [ ] Implement consolidateSession() method
   - [ ] Run consolidation in background goroutine

5. ⏳ **Phase 5: Configuration**
   - [ ] Add consolidation settings to .env
   - [ ] Add config validation
   - [ ] Document configuration options

6. ⏳ **Phase 6: Testing**
   - [ ] Unit tests for all new methods
   - [ ] Integration tests for end-to-end flow
   - [ ] Manual testing with real conversations
   - [ ] Performance testing (consolidation speed)

7. ⏳ **Phase 7: Documentation**
   - [ ] Update README.md with consolidation info
   - [ ] Add CONSOLIDATION.md design doc
   - [ ] Document configuration options
   - [ ] Add troubleshooting guide

## Success Criteria

- [ ] Sessions automatically consolidate when exceeding threshold
- [ ] Important facts are extracted to GlobalMemory.md
- [ ] Project-specific facts are extracted to project memory
- [ ] Conversation summaries appear in HISTORY.md
- [ ] LastConsolidated field is properly updated
- [ ] Consolidation runs in background without blocking user
- [ ] All tests pass
- [ ] Documentation is complete

## Design Decisions

### 1. Pruning vs Keeping Consolidated Messages

**Option A: Keep all messages** (recommended for v1)
- Pros: Full history available, can re-consolidate if needed
- Cons: Session files grow over time

**Option B: Prune consolidated messages**
- Pros: Smaller session files, faster loading
- Cons: Lose detailed history, can't re-consolidate

**Decision**: Start with Option A, add pruning as optional feature later

### 2. Consolidation Timing

**Option A: Synchronous** (block user until done)
- Pros: Guaranteed completion before next message
- Cons: User experiences delay

**Option B: Asynchronous** (background goroutine)
- Pros: No user-facing delay
- Cons: Consolidation might fail silently

**Decision**: Use Option B with proper error logging

### 3. LLM Model for Consolidation

**Option A: Use same model as main agent**
- Pros: Consistent quality
- Cons: Expensive, slower

**Option B: Use cheaper/faster model**
- Pros: Cost-effective, faster
- Cons: Lower quality summaries

**Decision**: Make it configurable, default to same model

### 4. Fact Extraction Format

**Option A: Structured JSON**
- Pros: Easy to parse, validate
- Cons: LLM might struggle with format

**Option B: Markdown with headers**
- Pros: Natural for LLM, human-readable
- Cons: Requires parsing logic

**Decision**: Use Option B (Markdown with headers)

## Future Enhancements

- [ ] **Semantic search** over HISTORY.md using embeddings
- [ ] **Manual consolidation command** (`/consolidate`)
- [ ] **Consolidation statistics** (how many facts extracted, when last run)
- [ ] **Smart consolidation** (consolidate less important messages more aggressively)
- [ ] **Consolidation review** (let user approve facts before adding to memory)
- [ ] **Multi-level consolidation** (consolidate HISTORY.md itself periodically)
- [ ] **Consolidation templates** (different prompts for different project types)
- [ ] **Consolidation metrics** (track quality, usefulness of extracted facts)

## Notes

- Consolidation is transparent to the user - happens automatically
- Failed consolidation should not break the agent - log errors and continue
- Consolidation prompt should be tuned based on real-world usage
- Consider rate limiting consolidation to avoid excessive LLM calls
- Monitor consolidation quality and adjust prompts as needed
