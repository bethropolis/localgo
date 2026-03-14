#!/bin/sh
# Pre-remove script for .deb and .rpm packages
set -e

if command -v systemctl >/dev/null 2>&1; then
    systemctl stop    localgo 2>/dev/null || true
    systemctl disable localgo 2>/dev/null || true
    systemctl daemon-reload   2>/dev/null || true
fi