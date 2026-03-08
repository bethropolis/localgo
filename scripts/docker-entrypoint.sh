#!/bin/sh
set -e

echo "LocalGo Docker Entrypoint"
echo "========================="

# ---------------------------------------------------------------------------
# 1. Ensure data directories exist.
#
#    Podman does NOT auto-create bind-mount source directories on the host;
#    Docker does.  This mkdir is harmless in either case.
# ---------------------------------------------------------------------------
mkdir -p /app/downloads /app/config/.security

# ---------------------------------------------------------------------------
# 2. Permission setup.
#
#    Three runtime cases:
#
#    A) Already non-root (podman run --user, or USER in image)
#       → exec directly; nothing to do.
#
#    B) Rootless Podman (default)
#       → uid_map shows container-UID-0 maps to a non-zero host UID.
#       → Process will run as UID 0, which IS the host user.
#       → Podman may have created bind-mount dirs owned by host-root (outside
#          the user namespace).  chown 0:0 inside the namespace re-maps them
#          to the host user on-disk, making them writable.
#       → No privilege drop — we're already the correct user.
#
#    C) Rootful Docker / Podman
#       → uid_map shows container-UID-0 maps to host UID 0 (real root).
#       → chown dirs to PUID:PGID, remap the in-container localgo user to
#          match, then drop privileges with su-exec.
# ---------------------------------------------------------------------------

if [ "$(id -u)" != "0" ]; then
    # Case A
    echo "Running as UID $(id -u) — no permission setup needed."
    echo "Starting LocalGo..."
    echo ""
    exec "$@"
fi

if [ "${SKIP_PERMS_FIX:-false}" = "true" ]; then
    echo "Skipping permission fix (SKIP_PERMS_FIX=true)."
    echo "Starting LocalGo..."
    echo ""
    exec "$@"
fi

# Read the host UID that container-UID-0 maps to.
HOST_UID_BASE=0
if [ -r /proc/self/uid_map ]; then
    HOST_UID_BASE=$(awk 'NR==1 {print $2}' /proc/self/uid_map)
fi

if [ "${HOST_UID_BASE:-0}" != "0" ]; then
    # Case B — rootless Podman.
    echo "Rootless Podman — container UID 0 = host UID ${HOST_UID_BASE}."
    echo "Fixing bind-mount ownership to container UID 0..."
    chown -R 0:0 /app/downloads /app/config
    echo "Starting LocalGo..."
    echo ""
    exec "$@"
fi

# Case C — rootful container.
PUID="${PUID:-1000}"
PGID="${PGID:-1000}"
echo "Rootful container — fixing ownership (UID:$PUID GID:$PGID)..."

# Remap the in-container localgo user to match PUID:PGID so that files
# written to bind-mounted volumes appear owned by the correct host user.
if [ "$(id -u localgo 2>/dev/null)" != "$PUID" ]; then
    sed -i "s/^localgo:x:[0-9]*:[0-9]*:/localgo:x:$PUID:$PGID:/" /etc/passwd 2>/dev/null || true
    sed -i "s/^localgo:x:[0-9]*:/localgo:x:$PGID:/"              /etc/group  2>/dev/null || true
fi

chown -R "$PUID:$PGID" /app/downloads /app/config

echo "Starting LocalGo..."
echo ""

# su-exec is installed in the image; gosu and su are kept as fallbacks for
# anyone running this script in a different base image.
if command -v su-exec >/dev/null 2>&1; then
    exec su-exec localgo "$@"
elif command -v gosu >/dev/null 2>&1; then
    exec gosu localgo "$@"
else
    exec su -s /bin/sh localgo -c "$*"
fi