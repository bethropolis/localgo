# Configuration Guide

LocalGo can be configured via Command Line Flags, Environment Variables, or a Config File.

## Precedence Order
1.  **Command Line Flags** (Highest priority)
2.  **Environment Variables**
3.  **Default Values** (Lowest priority)

---

## Command Line Flags

Flags are specific to each subcommand. Run `localgo help <command>` to see them, or see the [CLI Reference](CLI_REFERENCE.md).

### Global Flags
These can be passed before any subcommand.

| Flag | Description | Default |
|------|-------------|---------|
| `--verbose` | Enable debug logging | `false` |
| `--json` | Enable JSON log output | `false` |

### `serve` Flags
| Flag | Description | Default |
|------|-------------|---------|
| `--port` | TCP port to listen on | `53317` |
| `--http` | Disable HTTPS (use HTTP only) | `false` |
| `--alias` | Device name visible to others | from config |
| `--dir` | Directory to save incoming files | from config |
| `--pin` | Require PIN for incoming transfers | — |
| `--interval` | Discovery announcement interval in seconds | `30` |
| `--auto-accept` | Auto-accept incoming files without prompting | `false` |
| `--no-clipboard` | Save incoming text as a file instead of copying to clipboard | `false` |
| `--quiet` | Suppress non-essential output | `false` |
| `--verbose` | Enable debug logging | `false` |

### `share` Flags
| Flag | Description | Default |
|------|-------------|---------|
| `--file` | Path to file or directory to share (required, repeatable) | — |
| `--port` | TCP port to listen on | `53317` |
| `--http` | Disable HTTPS (use HTTP only) | `false` |
| `--alias` | Device name visible to others | from config |
| `--pin` | Require PIN for incoming transfers | — |
| `--auto-accept` | Auto-accept incoming files without prompting | `false` |
| `--no-clipboard` | Save incoming text as a file instead of copying to clipboard | `false` |

### `send` Flags
| Flag | Description | Default |
|------|-------------|---------|
| `--file` | Path to file or directory to send (required, repeatable) | — |
| `--to` | Exact alias of recipient (required) | — |
| `--port` | Target device port | auto-detect |
| `--timeout` | Transfer timeout in seconds | `30` |
| `--alias` | Sender alias | from config |

### `discover` Flags
| Flag | Description | Default |
|------|-------------|---------|
| `--timeout` | Discovery timeout in seconds | `5` |
| `--json` | Output results in JSON format | `false` |
| `--quiet` | Only show results, no status messages | `false` |

### `scan` Flags
| Flag | Description | Default |
|------|-------------|---------|
| `--timeout` | Scan timeout in seconds | `15` |
| `--port` | Port to scan | `53317` |
| `--json` | Output results in JSON format | `false` |
| `--quiet` | Only show results, no status messages | `false` |

### `devices` / `info` Flags
| Flag | Description | Default |
|------|-------------|---------|
| `--json` | Output results in JSON format | `false` |

---

## Environment Variables

You can set these globally to avoid repeating flags.

| Variable | Description | Default |
|----------|-------------|---------|
| `LOCALSEND_ALIAS` | Device name | Hostname |
| `LOCALSEND_PORT` | Port number | `53317` |
| `LOCALSEND_DOWNLOAD_DIR` | Save path for incoming files | `./downloads` |
| `LOCALSEND_SECURITY_DIR` | Security files path | (Auto-detected) |
| `LOCALSEND_PIN` | Security PIN | (Empty) |
| `LOCALSEND_FORCE_HTTP` | Disable HTTPS, use HTTP only | `false` |
| `LOCALSEND_DEVICE_TYPE` | Device type (`mobile`/`desktop`/`laptop`/`tablet`/`server`/`headless`/`web`/`other`) | `desktop` |
| `LOCALSEND_DEVICE_MODEL` | Device model string | `LocalGo` |
| `LOCALSEND_AUTO_ACCEPT` | Auto-accept incoming files (`true` or `1`) | `false` |
| `LOCALSEND_NO_CLIPBOARD` | Save incoming text as a file instead of clipboard (`true` or `1`) | `false` |
| `LOCALSEND_MULTICAST_GROUP` | Multicast IP address | `224.0.0.167` |
| `LOCALSEND_LOG_LEVEL` | Log verbosity (`debug`/`info`/`warn`/`error`) | `info` |

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
LOCALSEND_NO_CLIPBOARD=true
```

Then run:
```bash
source .env && localgo serve
```

---

## Technical Details

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
