# vibebot 🤖

A production-ready AI assistant bot written in Go with advanced features including project management, dynamic model switching, and automatic context consolidation.

## Why vibebot?

- **Full Control**: Clean Go implementation with no upstream dependencies
- **Production Ready**: Multi-user sessions, persistence, automatic consolidation
- **Project Isolation**: Separate memory contexts for different projects
- **Model Flexibility**: Runtime model switching with persistence
- **Clean Architecture**: Modular design with clear separation of concerns

## Architecture

```
vibebot/
├── cmd/vibebot/          # Main application entry point
├── internal/
│   ├── agent/            # Core agent loop and decision making
│   ├── consolidation/    # Automatic context consolidation
│   ├── memory/           # Persistent memory system with project support
│   ├── llm/              # LLM provider integrations (OpenRouter)
│   ├── tools/            # Tool/function calling system
│   ├── session/          # Per-user session management
│   ├── modelmanager/     # Dynamic model switching
│   ├── config/           # Configuration management
│   └── logger/           # Structured logging
├── pkg/
│   └── telegram/         # Telegram bot integration
└── workspace/            # Runtime workspace for the bot
    ├── memory/           # Global memory files
    ├── projects/         # Project-specific memory files
    └── sessions/         # Per-user session state
```

## Features

### ✅ Core Functionality
- [x] Core agent loop with iterative tool execution (max 40 iterations)
- [x] LLM provider integration (OpenRouter with multiple models)
- [x] Telegram gateway with user whitelisting
- [x] Multi-user session management with persistence
- [x] Clean, modular architecture

### ✅ Memory System
- [x] Two-layer memory (GlobalMemory.md + HISTORY.md)
- [x] Project-based memory isolation
- [x] Automatic conversation logging
- [x] Automatic context consolidation (at 100 messages)
- [x] Per-session project tracking

### ✅ Tools
- [x] File operations (read, write, edit, list)
- [x] Shell command execution (with safety guards)
- [x] Message sending (context-aware)

### ✅ Advanced Features
- [x] Dynamic model switching (runtime model selection)
- [x] Project management (create, switch, delete, list)
- [x] Session consolidation with fact extraction
- [x] Model persistence across restarts

### 📋 Planned
- [ ] Web search and fetch
- [ ] Cron/scheduling system
- [ ] Self-modification capabilities
- [ ] Discord gateway

## Getting Started

See [SETUP.md](SETUP.md) for detailed setup instructions.

Quick start:

```bash
# 1. Copy and configure environment
cp .env.example .env
# Edit .env with your tokens

# 2. Build and run
./run.sh
```

### Key Commands

**Conversation:**
- `/new` - Start new conversation (clears context)
- `/help` - Show help message

**Model Management:**
- `/model` or `/models` - Show current model
- `/model list` - List all available models
- `/model <number>` - Switch to model by number
- `/model <name>` - Switch to model by partial name

**Project Management:**
- `/projects` - List all projects
- `/project create <name>` - Create and switch to new project
- `/project <name>` - Switch to existing project
- `/project delete <name>` - Delete a project
- `/project clear` - Clear current project (use global context only)

## Setup

### Prerequisites
- Go 1.24 or later
- Telegram account
- OpenRouter API key (or other LLM provider)

### Quick Start

```bash
# 1. Clone and configure
git clone https://github.com/vpoluyaktov/vibebot.git
cd vibebot
cp .env.example .env
# Edit .env with your tokens

# 2. Run
./run.sh
```

### Configuration

Edit `.env` file:

```env
# Telegram Bot
TELEGRAM_TOKEN=your_bot_token_from_@BotFather
TELEGRAM_ALLOWED_USERS=your_telegram_user_id  # Get from @userinfobot

# LLM Provider
OPENROUTER_API_KEY=your_openrouter_api_key
OPENROUTER_MODEL=anthropic/claude-sonnet-4.5
OPENROUTER_ALLOWED_MODELS=anthropic/claude-sonnet-4.5,openai/gpt-4-turbo,google/gemini-pro-1.5

# Storage
WORKSPACE_DIR=/home/ubuntu/.vibebot/workspace
```

**Get Telegram Bot Token:**
1. Message [@BotFather](https://t.me/BotFather) on Telegram
2. Send `/newbot` and follow prompts
3. Copy the token to `.env`

**Get Your User ID:**
1. Message [@userinfobot](https://t.me/userinfobot)
2. Copy your ID to `TELEGRAM_ALLOWED_USERS`

**Get OpenRouter API Key:**
1. Sign up at [openrouter.ai](https://openrouter.ai/)
2. Create API key at [openrouter.ai/keys](https://openrouter.ai/keys)
3. Copy to `.env`

## Production Deployment

### Systemd Service (Linux)

```bash
# Install service
sudo cp vibebot.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable vibebot
sudo systemctl start vibebot

# View logs
journalctl -u vibebot -f
```

### Management Commands

```bash
# Start/stop/restart
sudo systemctl start vibebot
sudo systemctl stop vibebot
sudo systemctl restart vibebot

# Check status
sudo systemctl status vibebot

# View logs
./logs.sh  # Last 50 lines, follow mode
journalctl -u vibebot -n 100  # Last 100 lines
```

## Architecture

### Core Components

- **Agent** - Core decision loop with iterative tool execution (max 40 iterations)
- **Memory System** - Three-layer: GlobalMemory, Project Memory, History
- **Session Manager** - Per-user conversation state with persistence
- **Model Manager** - Dynamic model switching with persistence
- **Consolidator** - Automatic context consolidation at 100 messages
- **Tools** - File operations, shell execution, messaging
- **LLM Provider** - OpenRouter integration with multiple models
- **Telegram Gateway** - Bot API with user whitelisting

### Memory System

**GlobalMemory.md** - Cross-project facts, always loaded
**Project Memory** - Project-specific context, loaded when active
**HISTORY.md** - Append-only conversation log, searchable

### Project Management

Isolate different work contexts:

```bash
/project create myproject    # Create and switch to project
/project myproject           # Switch to existing project
/projects                    # List all projects
/project clear               # Use global context only
/project delete myproject    # Delete project
```

### Model Management

Switch models at runtime:

```bash
/model                       # Show current model
/model list                  # List available models
/model 1                     # Switch by number
/model gpt-4                 # Switch by partial name
```

Model selection persists across restarts.

## Safety Features

- **User Whitelisting** - Only allowed Telegram users can interact
- **Command Blocking** - Dangerous commands (rm -rf, format, etc.) blocked
- **Timeouts** - 60-second timeout on shell commands
- **Output Limits** - 10K character truncation
- **Path Safety** - Workspace-relative path resolution
- **Iteration Limits** - Max 40 tool iterations prevents infinite loops

## Development

### Building

```bash
go build -o vibebot cmd/vibebot/main.go
```

### Running

```bash
# With run script (recommended)
./run.sh

# Or manually
export $(grep -v '^#' .env | xargs)
./vibebot gateway
```

### Project Structure

```
vibebot/
├── cmd/vibebot/          # Main entry point
├── internal/
│   ├── agent/            # Core agent loop
│   ├── consolidation/    # Auto consolidation
│   ├── memory/           # Memory system
│   ├── session/          # Session management
│   ├── modelmanager/     # Model switching
│   ├── llm/              # LLM providers
│   ├── tools/            # Tool registry
│   ├── config/           # Configuration
│   └── logger/           # Logging
├── pkg/telegram/         # Telegram gateway
└── workspace/            # Runtime data
    ├── memory/           # Global memory
    ├── projects/         # Project memories
    └── sessions/         # User sessions
```

## Troubleshooting

### Bot doesn't respond
- Check logs: `journalctl -u vibebot -f`
- Verify bot is running: `systemctl status vibebot`
- Check Telegram token is valid
- Ensure your user ID is in whitelist

### LLM API errors
- Verify OpenRouter API key
- Check account has credits
- Try different model

### Permission errors
```bash
mkdir -p ~/.vibebot/workspace
chmod 755 ~/.vibebot/workspace
```

## License

MIT
