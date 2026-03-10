#!/bin/sh
# Post-install script for .deb and .rpm packages
set -e

# Reload systemd so it sees the new service file
if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
fi

echo "LocalGo installed."
echo ""
echo "To start as a system service:"
echo "  sudo systemctl enable --now localgo"
echo ""
echo "Edit /etc/localgo/localgo.env to configure alias, port, etc."
