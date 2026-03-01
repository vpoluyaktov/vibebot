# Timer Task Resumption Feature

## Overview
Implemented automatic task resumption when timers expire. The LLM is now automatically invoked to continue operations after long-running tasks complete.

## Architecture

### New Components

1. **Task Manager** (`internal/tasks/tasks.go`)
   - Manages pending tasks with persistence
   - Stores tasks to disk (`workspace/tasks/*.json`)
   - Schedules timers and triggers resumption
   - Recovers tasks on service restart
   - Thread-safe with sync.RWMutex

2. **Updated Timer Tool** (`internal/tools/timer.go`)
   - Now requires `task_context` parameter (what to do when timer expires)
   - Creates pending task instead of just sending notification
   - Integrates with task manager for automatic resumption
   - Optional `message` parameter for custom notifications

3. **Agent Task Resumption** (`internal/agent/agent.go`)
   - New `ResumeTask()` method
   - Automatically invoked when timer expires
   - Adds task context to conversation
   - Processes the resumed task via normal agent loop
   - Sends results to user automatically

### Data Flow

1. **Setting a Timer:**
   ```
   User → set_timer(duration, task_context) → Task Manager
   ↓
   Create PendingTask → Save to disk → Schedule goroutine
   ```

2. **Timer Expiration:**
   ```
   Goroutine wakes up → Task Manager resume handler
   ↓
   Agent.ResumeTask(chatID, task_context)
   ↓
   Add task context to session → ProcessMessage()
   ↓
   LLM processes task → Sends results to user
   ↓
   Delete task from storage
   ```

### File Structure

```
workspace/
├── tasks/                    # Pending tasks storage
│   ├── task_123_456.json    # Individual task files
│   └── task_789_012.json
├── memory/
│   ├── GlobalMemory.md
│   └── HISTORY.md
├── projects/
│   └── *.md
└── sessions/
    └── telegram_*.json
```

### Task Storage Format

```json
{
  "id": "task_123_1234567890",
  "chat_id": 123,
  "description": "⏰ Timer expired! Resuming task: Check if build completed",
  "context": "Check if build in /tmp/myproject completed successfully and report results",
  "expires_at": "2026-03-01T19:00:00Z",
  "created_at": "2026-03-01T18:55:00Z"
}
```

## Usage Examples

### Example 1: Build Monitoring
```
User: "Start the build and let me know when it's done"
LLM: [starts build]
LLM: set_timer(duration=600, task_context="Check if build in /tmp/myproject completed successfully and report results")
→ Timer set for 10 minutes
→ User disconnects

[10 minutes later]
→ Timer expires
→ LLM automatically invoked
→ LLM checks build status
→ LLM sends report to user
User: [receives notification with build results]
```

### Example 2: Download and Extract
```
User: "Download this large file and extract it when ready"
LLM: [starts download in background]
LLM: set_timer(duration=1800, task_context="Verify download of file.tar.gz completed, extract it, and summarize contents")
→ Timer set for 30 minutes

[30 minutes later]
→ Timer expires
→ LLM checks download
→ LLM extracts archive
→ LLM reports results
```

### Example 3: Reminder
```
User: "Remind me about the meeting in 1 hour"
LLM: set_timer(duration=3600, task_context="Remind user about the scheduled meeting", message="Meeting reminder!")
→ Timer set for 1 hour

[1 hour later]
→ Timer expires
→ LLM sends reminder message
→ User receives notification
```

## Updated System Prompt

The system prompt now includes comprehensive documentation about the timer tool:

- How automatic task resumption works
- When to use timers (builds, downloads, scheduled tasks)
- Parameter requirements (duration, task_context)
- Workflow explanation (4-step process)
- Persistence behavior (survives disconnections, lost on restart)

## Integration Points

1. **main.go** - Task manager initialization and resume handler setup
2. **agent.go** - ResumeTask method and task manager integration
3. **timer.go** - Updated tool definition with task_context parameter
4. **system_prompt.go** - Updated documentation for LLM guidance

## Limitations

1. **Service Restart**: Tasks are lost if vibebot service restarts (rare)
2. **Sequential Execution**: Task resumption uses same agent loop, no parallel execution
3. **No Cancellation**: Once set, timers cannot be cancelled (future enhancement)
4. **No List Command**: No /timers command to view active timers (future enhancement)

## Future Enhancements

1. Add `/timers` command to list active timers
2. Add `/timer cancel <id>` to cancel specific timers
3. Persist timers with cron-like scheduling for restart recovery
4. Add timer edit/reschedule functionality
5. Add recurring timers (e.g., every hour)
6. Add timer groups (cancel all timers for a task)

## Testing

To test the feature:

1. Ask the bot to start a long-running task (e.g., build, download)
2. Bot should set a timer with appropriate task_context
3. Wait for timer to expire (or use short duration for testing)
4. Verify bot automatically resumes and reports results
5. Test disconnection during timer (should still work)

## Notes

- Timers persist in memory and on disk
- Task resumption is non-blocking (uses goroutines)
- Tasks are automatically cleaned up after completion
- System supports multiple concurrent timers
- Each timer is isolated per chat (no cross-chat interference)
