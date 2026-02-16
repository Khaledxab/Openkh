#!/bin/bash
cd /home/khale/opencode-bot-go

# Kill any existing bot processes
pkill -f 'openkh' 2>/dev/null || true
pkill -f 'opencode-bot' 2>/dev/null || true
sleep 1

# Clean up old log
rm -f bot.log

# Export all variables from .env file
set -a
source .env
set +a

# Start the bot
nohup ./bin/openkh > bot.log 2>&1 &

# Wait a moment and check if it started
sleep 2
if pgrep -f 'openkh' > /dev/null; then
    echo "Bot started successfully"
    tail -10 bot.log
else
    echo "Bot failed to start"
    cat bot.log
    exit 1
fi
