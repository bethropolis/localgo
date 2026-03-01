# Configuration Guide

LocalGo can be configured via Command Line Flags, Environment Variables, or a Config File.

## Precedence Order
1.  **Command Line Flags** (Highest priority)
2.  **Environment Variables**
3.  **Default Values** (Lowest priority)

---

## 🚩 Command Line Flags

Flags are specific to each command. Use `localgo-cli <command> --help` to see them all.

### Common Flags
| Flag | Description | Default |
|------|-------------|---------|
| `--port` | TCP/UDP port to listen/scan on | `53317` |
| `--verbose` | Enable debug logging | `false` |
| `--quiet` | Suppress non-essential output | `false` |
| `--json` | Output results in JSON format | `false` |

### `serve` Flags
| Flag | Description | Example |
|------|-------------|---------|
| `--alias` | Device name visible to others | `--alias "FileServer"` |
| `--dir` | Directory to save incoming files | `--dir "/mnt/storage"` |
| `--pin` | Require PIN for incoming transfers | `--pin "9999"` |
| `--http` | Disable HTTPS (use HTTP only) | `--http` |
| `--auto-accept` | Auto-accept incoming files without prompting | `--auto-accept` |
| `--interval` | Discovery announcement interval in seconds | `--interval 60` |

### `send` Flags
| Flag | Description | Example |
|------|-------------|---------|
| `--file` | Path to file to send (Required) | `--file "./doc.pdf"` |
| `--to` | Exact alias of recipient (Required) | `--to "MyPhone"` |
| `--timeout` | Transfer timeout in seconds | `--timeout 60` |

### `share` Flags
| Flag | Description | Example |
|------|-------------|---------|
| `--file` | Path to file to share (Required, can be repeated) | `--file "./doc.pdf"` |
| `--alias` | Device name visible to others | `--alias "FileServer"` |
| `--pin` | Require PIN for incoming transfers | `--pin "9999"` |
| `--http` | Disable HTTPS (use HTTP only) | `--http` |
| `--auto-accept` | Auto-accept incoming files without prompting | `--auto-accept` |

---

## 🌍 Environment Variables

You can set these globally to avoid repeating flags.

| `LOCALSEND_ALIAS` | Device name | Hostname |
| `LOCALSEND_PORT` | Port number | `53317` |
| `LOCALSEND_DOWNLOAD_DIR` | Save path | `./downloads` |
| `LOCALSEND_SECURITY_DIR` | Security files path | (Auto-detected) |
| `LOCALSEND_PIN` | Security PIN | (Empty) |
| `LOCALSEND_MULTICAST_GROUP`| Multicast IP | `224.0.0.167` |
| `LOCALSEND_FORCE_HTTP` | Disable HTTPS, use HTTP only | `false` |
| `LOCALSEND_DEVICE_TYPE` | Device type (mobile/desktop/web/headless/server/laptop/tablet/other) | `server` |
| `LOCALSEND_DEVICE_MODEL` | Device model string | `LocalGo` |
| `LOCALSEND_AUTO_ACCEPT` | Auto-accept incoming files without prompting | `false` |
| `LOCALSEND_LOG_LEVEL` | Log verbosity | `info` |

### Docker-specific Variables
| Variable | Description | Default |
|---|---|---|
| `PUID` | User ID for file ownership in container | `1000` |
| `PGID` | Group ID for file ownership in container | `1000` |

**Example `.env` file:**
```bash
LOCALSEND_ALIAS="BackupServer"
LOCALSEND_DOWNLOAD_DIR="/raid/backups"
LOCALSEND_PIN="secure_pin_123"
```

Then run:
```bash
source .env && localgo-cli serve
```

---

## 🔧 Technical Details

### Security Directory

LocalGo uses XDG-compliant paths for storing TLS certificates and fingerprints.

**Directory resolution priority:**
1. `$LOCALSEND_SECURITY_DIR` (if set - explicit override)
2. `$XDG_CONFIG_HOME/localgo/.security` (Linux/Unix XDG standard)
3. `$HOME/.config/localgo/.security` (XDG default when XDG_CONFIG_HOME not set)
4. `$APPDATA/localgo/.security` (Windows)
5. `$HOME/.localgo/.security` (fallback)
6. `./.localgo_security` (legacy compatibility - executable directory)

The security directory contains:
- `context.json` - TLS certificate, private key, and fingerprint

**Migration from legacy location:**

If you have an existing `.localgo_security` directory in the executable directory, LocalGo will continue to use it for backward compatibility. To migrate to the XDG-compliant location:

```bash
# Create config directory
mkdir -p ~/.config/localgo

# Move security directory
mv .localgo_security ~/.config/localgo/.security

# Restart LocalGo - it will now use the new location
```

**Important:** Do not share the `context.json` file, as it contains your private key.

### Network Ports
- **TCP 53317**: Main HTTP/S server for file transfers.
- **UDP 53317**: Multicast listening for discovery.

*Ensure these ports are allowed through your firewall.*
