#!/bin/bash
# Self-restart script for vibebot
# Uses systemd service for clean restart

echo "Restarting vibebot via systemd..."
sudo systemctl restart vibebot

echo "Vibebot restart initiated."
exit 0
