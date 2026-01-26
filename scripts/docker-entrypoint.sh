#!/bin/sh
set -e

# Docker entrypoint script for LocalGo
# Automatically fixes volume permissions on startup

echo "LocalGo Docker Entrypoint"
echo "========================="

# Fix permissions for mounted volumes (running as root)
# We use chmod 777 to ensure the localgo user (and host user) can read/write
# regardless of UID mismatches between host and container
if [ -d "/app/downloads" ]; then
    echo "Fixing permissions for /app/downloads..."
    chown -R localgo:localgo /app/downloads || echo "  Warning: chown failed for /app/downloads"
    chmod -R 777 /app/downloads || echo "  Warning: chmod failed for /app/downloads"
fi

if [ -d "/app/config" ]; then
    echo "Fixing permissions for /app/config..."
    chown -R localgo:localgo /app/config || echo "  Warning: chown failed for /app/config"
    chmod -R 777 /app/config || echo "  Warning: chmod failed for /app/config"
fi

echo "Permissions fixed. Starting LocalGo as user 'localgo'..."
echo ""

# Switch to localgo user and execute the command
exec "$@"
