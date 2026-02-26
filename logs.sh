#!/bin/bash
# View vibebot logs from systemd journal

# Default: show last 50 lines and follow
journalctl -u vibebot -n 50 -f
