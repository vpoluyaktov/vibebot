# Setup Guide

## Prerequisites

- Go 1.21 or later
- Telegram account
- OpenRouter API key

## Step 1: Get Telegram Bot Token

1. Open Telegram and search for `@BotFather`
2. Send `/newbot` and follow the instructions
3. Copy the bot token (looks like `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`)

## Step 2: Get OpenRouter API Key

1. Go to https://openrouter.ai/
2. Sign up or log in
3. Go to Keys section
4. Create a new API key
5. Copy the key (starts with `sk-or-v1-`)

## Step 3: Configure Environment

```bash
# Copy the example env file
cp .env.example .env

# Edit .env and add your tokens
nano .env
```

## Step 4: Build and Run

```bash
# Build
go build -o vibebot cmd/vibebot/main.go

# Run
export $(cat .env | xargs)
./vibebot gateway
```

## Step 5: Test

1. Open Telegram
2. Search for your bot by username
3. Send a message
4. The bot should respond!

## Running as a Service (Optional)

Create `/etc/systemd/system/vibebot.service`:

```ini
[Unit]
Description=vibebot AI Assistant
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/path/to/vibebot
EnvironmentFile=/path/to/vibebot/.env
ExecStart=/path/to/vibebot/vibebot gateway
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Then:

```bash
sudo systemctl daemon-reload
sudo systemctl enable vibebot
sudo systemctl start vibebot
sudo systemctl status vibebot
```

## Troubleshooting

### Bot doesn't respond

- Check that the bot token is correct
- Check that the OpenRouter API key is valid
- Check logs for errors

### "TELEGRAM_TOKEN is required" error

- Make sure you've exported the environment variables
- Or use: `export $(cat .env | xargs) && ./vibebot gateway`

### Memory not persisting

- Check that WORKSPACE_DIR is writable
- Default location: `~/.vibebot/workspace/memory/`
