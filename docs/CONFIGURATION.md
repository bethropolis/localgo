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

### `send` Flags
| Flag | Description | Example |
|------|-------------|---------|
| `--file` | Path to file to send (Required) | `--file "./doc.pdf"` |
| `--to` | Exact alias of recipient (Required) | `--to "MyPhone"` |
| `--timeout` | Transfer timeout in seconds | `--timeout 60` |

---

## 🌍 Environment Variables

You can set these globally to avoid repeating flags.

| Variable | Description | Default |
|----------|-------------|---------|
| `LOCALSEND_ALIAS` | Device name | Hostname |
| `LOCALSEND_PORT` | Port number | `53317` |
| `LOCALSEND_DOWNLOAD_DIR` | Save path | `./downloads` |
| `LOCALSEND_PIN` | Security PIN | (Empty) |
| `LOCALSEND_MULTICAST_GROUP`| Multicast IP | `224.0.0.167` |
| `LOCALSEND_LOG_LEVEL` | Log verbosity | `info` |

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

### Security Context
LocalGo stores generated TLS certificates and config cache in:
- **Linux/Mac**: Directory of the executable + `/.localgo_security/`
- **Windows**: Directory of the executable + `\.localgo_security\`

*Warning: Do not share the `context.json` file inside this directory, as it contains your private key.*

### Network Ports
- **TCP 53317**: Main HTTP/S server for file transfers.
- **UDP 53317**: Multicast listening for discovery.

*Ensure these ports are allowed through your firewall.*
