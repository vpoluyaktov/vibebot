#!/bin/bash
set -e

# Load environment variables
if [ ! -f .env ]; then
    echo "Error: .env file not found"
    echo "Copy .env.example to .env and configure it"
    exit 1
fi

export $(grep -v '^#' .env | xargs)

# Kill any existing vibebot instances
echo "Checking for existing vibebot instances..."
if pgrep -f "vibebot gateway" > /dev/null; then
    echo "Found running vibebot instance(s), stopping them..."
    pkill -9 -f "vibebot gateway"
    sleep 1
    echo "Old instances stopped."
else
    echo "No existing instances found."
fi

# Build if binary doesn't exist
if [ ! -f vibebot ]; then
    echo "Building vibebot..."
    /usr/local/go/bin/go build -o vibebot cmd/vibebot/main.go
fi

# Run the bot
echo "Starting vibebot..."
echo "Bot: @strangervp_vibebot"
echo "Model: $OPENROUTER_MODEL"
echo "Workspace: $WORKSPACE_DIR"
echo ""
./vibebot gateway
