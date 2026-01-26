#!/bin/sh
set -e

# Docker entrypoint script for LocalGo
echo "LocalGo Docker Entrypoint"
echo "========================="

# define user and group ids
PUID=${PUID:-1000}
PGID=${PGID:-1000}

# Update localgo user UID/GID if they don't match environment variables
# This allows matching host permissions
if [ "$(id -u localgo)" != "$PUID" ]; then
    sed -i "s/^localgo:x:[0-9]*:[0-9]*:/localgo:x:$PUID:$PGID:/" /etc/passwd
    sed -i "s/^localgo:x:[0-9]*:/localgo:x:$PGID:/" /etc/group
fi

# Fix permissions for mounted volumes (running as root)
echo "Fixing permissions (UID:$PUID GID:$PGID)..."

if [ -d "/app/downloads" ]; then
    chown -R localgo:localgo /app/downloads
fi

if [ -d "/app/config" ]; then
    chown -R localgo:localgo /app/config
fi

echo "Starting LocalGo as user 'localgo'..."
echo ""

# Execute the command as the localgo user
exec su-exec localgo "$@"