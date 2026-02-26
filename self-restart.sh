#!/bin/bash
# Self-restart script for vibebot
# This script spawns a completely independent process that survives parent death

# Write restart command to a temporary script
cat > /tmp/vibebot_restart_worker.sh << 'EOF'
#!/bin/bash
# Wait for parent to exit
sleep 3

# Kill old instances
pkill -9 -f "vibebot gateway" 2>/dev/null
sleep 1

# Start new instance
cd /mnt/hostgit/vibebot
./run.sh > /tmp/vibebot.log 2>&1 &

echo "Vibebot restarted at $(date)"
EOF

chmod +x /tmp/vibebot_restart_worker.sh

# Execute the worker script in a completely detached way
# Using setsid to create a new session, making it independent of parent
setsid /tmp/vibebot_restart_worker.sh > /tmp/vibebot_restart.log 2>&1 &

echo "Restart scheduled. Vibebot will restart in 3 seconds."
exit 0
