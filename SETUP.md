# Setup Guide

This guide will walk you through setting up vibebot from scratch.

## Prerequisites

- Go 1.21 or later
- A Telegram account
- An OpenRouter account (or other LLM provider)

## Step 1: Create a Telegram Bot

1. Open Telegram and message [@BotFather](https://t.me/BotFather)
2. Send the command `/newbot`
3. Follow the prompts:
   - Choose a name for your bot (e.g., "My Vibebot")
   - Choose a username (must end in 'bot', e.g., "myvibebot")
4. BotFather will give you a token like: `8333040078:AAGsTh7Mi5jq8zFJJP-S-HkM99KnN79ErRk`
5. **Save this token** - you'll need it in Step 3

## Step 2: Get an OpenRouter API Key

1. Go to [OpenRouter](https://openrouter.ai/)
2. Sign up or log in
3. Navigate to [API Keys](https://openrouter.ai/keys)
4. Click "Create Key"
5. **Save this key** - it looks like: `sk-or-v1-...`

### Alternative: Use Anthropic or OpenAI Directly

If you prefer to use Anthropic or OpenAI directly instead of OpenRouter:

- **Anthropic**: Get API key from [console.anthropic.com](https://console.anthropic.com/)
- **OpenAI**: Get API key from [platform.openai.com](https://platform.openai.com/)

Note: You'll need to implement the provider in `internal/llm/` (see Architecture docs).

## Step 3: Configure vibebot

1. Clone the repository:
```bash
git clone https://github.com/vpoluyaktov/vibebot.git
cd vibebot
```

2. Copy the environment template:
```bash
cp .env.example .env
```

3. Edit `.env` with your credentials:
```bash
nano .env  # or use your preferred editor
```

4. Update these values:
```env
TELEGRAM_TOKEN=your_telegram_bot_token_here
TELEGRAM_ALLOWED_USERS=your_telegram_user_id
OPENROUTER_API_KEY=your_openrouter_api_key_here
OPENROUTER_MODEL=anthropic/claude-sonnet-4.5
WORKSPACE_DIR=/home/ubuntu/.vibebot/workspace
```

**Finding Your Telegram User ID:**
1. Message [@userinfobot](https://t.me/userinfobot) on Telegram
2. It will reply with your user ID (e.g., `339899302`)
3. Add this ID to `TELEGRAM_ALLOWED_USERS`
4. For multiple users, separate with commas: `339899302,123456789`
5. Leave empty to allow all users (not recommended)

### Configuration Options

| Variable | Description | Default |
|----------|-------------|---------|
| `TELEGRAM_TOKEN` | Bot token from @BotFather | Required |
| `TELEGRAM_ALLOWED_USERS` | Comma-separated user IDs | Empty (allow all) |
| `OPENROUTER_API_KEY` | OpenRouter API key | Required |
| `OPENROUTER_MODEL` | Model to use | `anthropic/claude-3.5-sonnet` |
| `WORKSPACE_DIR` | Where bot stores data | `~/.vibebot/workspace` |

### Available Models

See [OpenRouter Models](https://openrouter.ai/models) for the full list. Popular choices:

- `anthropic/claude-sonnet-4.5` - Latest Claude (recommended)
- `anthropic/claude-3.5-sonnet` - Previous Claude version
- `openai/gpt-4-turbo` - GPT-4 Turbo
- `google/gemini-pro-1.5` - Google Gemini
- `meta-llama/llama-3.1-70b-instruct` - Open source option

## Step 4: Build and Run

### Option A: Using the run script (easiest)

```bash
./run.sh
```

This will:
- Check for `.env` file
- Build the binary if needed
- Start the bot

### Option B: Manual build and run

```bash
# Build
go build -o vibebot cmd/vibebot/main.go

# Load environment and run
export $(grep -v '^#' .env | xargs)
./vibebot gateway
```

### Option C: Run without building

```bash
# Load environment
export $(grep -v '^#' .env | xargs)

# Run directly
go run cmd/vibebot/main.go gateway
```

## Step 5: Test Your Bot

1. Open Telegram
2. Search for your bot by username (e.g., `@myvibebot`)
3. Start a conversation with `/start`
4. Try asking: "Can you read the README.md file?"

The bot should respond and execute the `read_file` tool!

## Troubleshooting

### "Error: .env file not found"

Make sure you copied `.env.example` to `.env`:
```bash
cp .env.example .env
```

### "Error: TELEGRAM_TOKEN not set"

Edit your `.env` file and add your bot token from @BotFather.

### "Error: failed to initialize Telegram gateway"

- Check that your token is correct
- Make sure there are no extra spaces in the `.env` file
- Verify the token is still valid (check with @BotFather)

### "Error: LLM API error"

- Check that your OpenRouter API key is correct
- Verify you have credits in your OpenRouter account
- Try a different model (some require special access)

### Bot doesn't respond

- Check the console for errors
- Verify the bot is running (`ps aux | grep vibebot`)
- Make sure you're messaging the correct bot
- Check that the bot isn't blocked by Telegram

### Permission errors

Make sure the workspace directory is writable:
```bash
mkdir -p ~/.vibebot/workspace
chmod 755 ~/.vibebot/workspace
```

## Running as a Service

### systemd (Linux)

Create `/etc/systemd/system/vibebot.service`:

```ini
[Unit]
Description=Vibebot AI Assistant
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/vibebot
EnvironmentFile=/home/ubuntu/vibebot/.env
ExecStart=/home/ubuntu/vibebot/vibebot gateway
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable vibebot
sudo systemctl start vibebot
sudo systemctl status vibebot
```

View logs:
```bash
sudo journalctl -u vibebot -f
```

## Next Steps

- Read [ARCHITECTURE.md](docs/ARCHITECTURE.md) to understand how vibebot works
- Read [MEMORY.md](docs/MEMORY.md) to learn about the memory system
- Try asking the bot to help you with tasks
- Explore the available tools and capabilities

## Security Notes

- **Never commit `.env`** - It's in `.gitignore` by default
- **Keep your tokens secret** - Don't share them publicly
- **Use user whitelisting** - Set `TELEGRAM_ALLOWED_USERS` to restrict access
- **Review command execution** - The bot can run shell commands
- **Monitor API usage** - Check your OpenRouter/LLM provider bills
- **Unauthorized access** - Users not in whitelist will see their ID and be blocked

## Getting Help

- Check the [documentation](docs/)
- Review the [architecture](docs/ARCHITECTURE.md)
- Look at example conversations in your `workspace/memory/HISTORY.md`
- Open an issue on GitHub
