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

## Features (Planned)

- [x] Project initialization
- [ ] Core agent loop
- [ ] LLM provider integration (OpenAI, Anthropic, etc.)
- [ ] Tool/function calling system
- [ ] Telegram gateway
- [ ] Two-layer memory system (MEMORY.md + HISTORY.md)
- [ ] File operations (read, write, edit)
- [ ] Shell command execution
- [ ] Web search and fetch
- [ ] Cron/scheduling system
- [ ] Self-modification capabilities

## Getting Started

```bash
# Build
go build -o vibebot cmd/vibebot/main.go

# Run
./vibebot gateway
```

## Development

This is a fresh implementation inspired by nanobot but built from the ground up in Go.

## License

MIT
