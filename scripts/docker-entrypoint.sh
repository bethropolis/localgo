#!/bin/sh
set -e

# Docker entrypoint script for LocalGo
echo "LocalGo Docker Entrypoint"
echo "========================="

# Skip permission fixing if already running as non-root (rootless Podman)
# or if explicitly disabled via environment variable
SKIP_PERMS_FIX="${SKIP_PERMS_FIX:-false}"

if [ "$SKIP_PERMS_FIX" = "true" ]; then
    echo "Skipping permission fix (SKIP_PERMS_FIX=true)..."
elif [ "$(id -u)" != "0" ]; then
    echo "Running as non-root user, skipping permission fix..."
else
    # define user and group ids
    PUID=${PUID:-1000}
    PGID=${PGID:-1000}

    # Update localgo user UID/GID if they don't match environment variables
    # This allows matching host permissions
    if [ "$(id -u localgo 2>/dev/null)" != "$PUID" ]; then
        sed -i "s/^localgo:x:[0-9]*:[0-9]*:/localgo:x:$PUID:$PGID:/" /etc/passwd 2>/dev/null || true
        sed -i "s/^localgo:x:[0-9]*:/localgo:x:$PGID:/" /etc/group 2>/dev/null || true
    fi

    # Fix permissions for mounted volumes (running as root)
    echo "Fixing permissions (UID:$PUID GID:$PGID)..."

    if [ -d "/app/downloads" ]; then
        chown -R localgo:localgo /app/downloads
    fi

    if [ -d "/app/config" ]; then
        chown -R localgo:localgo /app/config
    fi
fi

echo "Starting LocalGo..."
echo ""

# Determine the best way to execute as non-root user
# Try su-exec first (standard), fall back to gosu, then direct exec
if [ "$(id -u)" = "0" ]; then
    if command -v su-exec >/dev/null 2>&1; then
        exec su-exec localgo "$@"
    elif command -v gosu >/dev/null 2>&1; then
        exec gosu localgo "$@"
    else
        # Fallback: use su with shell substitution
        # This is less secure but works when neither su-exec nor gosu is available
        echo "Warning: su-exec/gosu not found, using fallback method..."
        exec su -s /bin/sh localgo -c "$*"
    fi
else
    # Already running as non-root (rootless Podman)
    exec "$@"
fi
