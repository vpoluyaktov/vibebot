# vibebot 🤖

A modern AI assistant bot written in Go with full control and extensibility.

## Why vibebot?

- **Full Control**: No dependency on upstream repos with 500+ unmerged PRs
- **Go-First**: Built with Go for performance, simplicity, and maintainability
- **Self-Improving**: The bot can modify and improve its own codebase
- **Clean Architecture**: Modular design with clear separation of concerns

## Architecture

```
vibebot/
├── cmd/vibebot/          # Main application entry point
├── internal/
│   ├── agent/            # Core agent loop and decision making
│   ├── gateway/          # Message routing and channel management
│   ├── memory/           # Persistent memory system
│   ├── llm/              # LLM provider integrations
│   └── tools/            # Tool/function calling system
├── pkg/
│   ├── telegram/         # Telegram bot integration
│   └── discord/          # Discord bot integration (future)
└── workspace/            # Runtime workspace for the bot
    └── memory/           # Persistent memory files
```

## Features

- [x] Project initialization
- [x] Core agent loop with iterative tool execution
- [x] LLM provider integration (OpenRouter)
- [x] Telegram gateway
- [x] Two-layer memory system (MEMORY.md + HISTORY.md)
- [x] Clean, modular architecture
- [x] Tool/function calling system
- [x] File operations (read, write, edit, list)
- [x] Shell command execution (with safety guards)
- [ ] Web search and fetch
- [ ] Cron/scheduling system
- [ ] Self-modification capabilities
- [ ] Multi-user session management

## Getting Started

See [SETUP.md](SETUP.md) for detailed setup instructions.

Quick start:

```bash
# 1. Copy and configure environment
cp .env.example .env
# Edit .env with your tokens

# 2. Build
go build -o vibebot cmd/vibebot/main.go

# 3. Run
export $(cat .env | xargs)
./vibebot gateway
```

## Development

This is a fresh implementation inspired by nanobot but built from the ground up in Go.

## License

MIT
