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

1. **Phase 1: Memory System** (internal/memory/memory.go)
   - Add projects directory support
   - Implement project CRUD operations
   - Add project listing

2. **Phase 2: Session Integration** (internal/session/session.go)
   - Add CurrentProject field
   - Persist project in session JSON

3. **Phase 3: Agent Integration** (internal/agent/agent.go)
   - Load project memory in ProcessMessage
   - Update system message construction
   - Enhance project command handlers

4. **Phase 4: System Prompt** (internal/agent/system_prompt.go)
   - Add project-aware instructions
   - Document project commands

5. **Phase 5: Testing**
   - Unit tests
   - Integration tests
   - Manual testing

6. **Phase 6: Documentation**
   - Update README.md
   - Add project workflow examples
   - Document best practices

## Success Criteria

- [ ] Users can create, switch, delete, and list projects
- [ ] Project-specific memory is isolated from global memory
- [ ] Current project persists across sessions
- [ ] LLM receives both global and project context
- [ ] Project commands work without calling LLM
- [ ] All tests pass
- [ ] Documentation is complete

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
