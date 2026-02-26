# Systemd Service Setup

Vibebot runs as a systemd service for reliable production deployment.

## Installation

```bash
# Copy service file
sudo cp vibebot.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Enable auto-start on boot
sudo systemctl enable vibebot

# Start the service
sudo systemctl start vibebot
```

## Management

```bash
# Start service
sudo systemctl start vibebot

# Stop service
sudo systemctl stop vibebot

# Restart service
sudo systemctl restart vibebot

# Check status
sudo systemctl status vibebot

# View logs (last 50 lines, follow)
./logs.sh
# or
journalctl -u vibebot -f

# View logs (last 100 lines)
journalctl -u vibebot -n 100

# View logs since boot
journalctl -u vibebot -b
```

## Self-Restart from Vibebot

When vibebot needs to restart itself (e.g., after code changes), use:

```bash
./self-restart.sh
```

This script uses `systemctl restart` which is safe to call from within vibebot.

## Service Configuration

- **Service file**: `/etc/systemd/system/vibebot.service`
- **User**: ubuntu
- **Working directory**: `/mnt/hostgit/vibebot`
- **Environment**: Loaded from `.env` file
- **Auto-restart**: Yes (RestartSec=10)
- **Logging**: systemd journal

## Benefits

1. **Auto-start on boot** - Service starts automatically when system boots
2. **Auto-restart on crash** - Systemd restarts vibebot if it crashes
3. **Centralized logging** - All logs in systemd journal
4. **Clean process management** - No orphaned processes
5. **Safe self-restart** - Can restart from within vibebot without issues

## Troubleshooting

### Service won't start

```bash
# Check service status
sudo systemctl status vibebot

# View detailed logs
journalctl -u vibebot -n 100 --no-pager

# Check if binary exists
ls -la /mnt/hostgit/vibebot/vibebot

# Check if .env file exists
ls -la /mnt/hostgit/vibebot/.env
```

### Environment variables not loading

Make sure `.env` file exists and has correct permissions:

```bash
chmod 644 /mnt/hostgit/vibebot/.env
```

### Logs not appearing

Logs go to systemd journal, not to files. Use:

```bash
journalctl -u vibebot -f
```

## Development Workflow

1. **Make code changes** in `.go` files
2. **Build binary**: `/usr/local/go/bin/go build -o vibebot cmd/vibebot/main.go`
3. **Restart service**: `./self-restart.sh` or `sudo systemctl restart vibebot`

For config changes (`.env` only):
1. **Edit `.env` file**
2. **Restart service**: `./self-restart.sh` or `sudo systemctl restart vibebot`
