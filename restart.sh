#!/bin/bash
# Safe restart script that can be called from within vibebot
# This script detaches itself from the parent process to avoid being killed

# Detach from parent and run in background
(
    # Wait a moment for the calling process to finish
    sleep 2
    
    # Now kill old instances
    echo "Stopping old vibebot instances..." >> /tmp/vibebot_restart.log
    pkill -9 -f "vibebot gateway" 2>> /tmp/vibebot_restart.log
    sleep 1
    
    # Change to vibebot directory
    cd /mnt/hostgit/vibebot
    
    # Load environment
    export $(grep -v '^#' .env | xargs)
    
    # Start new instance
    echo "Starting new vibebot instance..." >> /tmp/vibebot_restart.log
    ./vibebot gateway >> /tmp/vibebot.log 2>&1 &
    
    echo "Restart complete at $(date)" >> /tmp/vibebot_restart.log
) &

# Exit immediately so the calling process can finish
exit 0
