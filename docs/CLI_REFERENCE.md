# CLI Reference

## `localgo serve`

Starts the receiver server. It runs in the foreground and accepts incoming file transfers and clipboard text from LocalSend-compatible devices.

**Usage:**
```bash
localgo serve [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--port` | int | from config | Port to run the server on |
| `--http` | bool | false | Use HTTP instead of HTTPS |
| `--pin` | string | — | PIN for authentication |
| `--alias` | string | from config | Device alias visible to others |
| `--dir` | string | from config | Directory to save incoming files |
| `--interval` | int | 30 | Discovery announcement interval in seconds |
| `--auto-accept` | bool | false | Auto-accept incoming files without prompting |
| `--no-clipboard` | bool | false | Save incoming text as a file instead of copying to clipboard |
| `--quiet` | bool | false | Quiet mode — minimal output |
| `--verbose` | bool | false | Verbose mode — detailed debug output |
| `--history` | string | ~/.local/share/localgo/history.jsonl | Path to transfer history JSONL file |
| `--exec` | string | — | Shell command to execute after each received file |

**Exec Hook Placeholders:**
| Placeholder | Description |
|-------------|-------------|
| `%f` | Absolute file path |
| `%n` | File name |
| `%s` | File size in bytes |
| `%a` | Sender alias |
| `%i` | Sender IP |

**Examples:**
```bash
localgo serve --exec "notify-send 'Got: %f'"
localgo serve --exec "curl -F 'file=@%f' https://example.com/upload"
```

**Behavior:**
- Starts HTTP/S server on port 53317 (or configured port).
- Joins Multicast group to listen for discovery announcements.
- Accepts upload requests; files are saved to `LOCALSEND_DOWNLOAD_DIR`.
- Incoming `text/plain` transfers are copied to the system clipboard by default (use `--no-clipboard` to save as a file instead).
- To stop, press `Ctrl+C`.

---

## `localgo share`

Shares files so other devices can download them. Announces itself over multicast with `Download: true`.

**Usage:**
```bash
localgo share --file FILE [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file` | string | — | File or directory to share (required, can be repeated) |
| `--port` | int | from config | Port to run the server on |
| `--http` | bool | false | Use HTTP instead of HTTPS |
| `--pin` | string | — | Require PIN for incoming transfers |
| `--alias` | string | from config | Device alias |
| `--auto-accept` | bool | false | Auto-accept incoming files without prompting |
| `--no-clipboard` | bool | false | Save incoming text as a file instead of copying to clipboard |
| `--history` | string | — | Path to transfer history JSONL file |
| `--exec` | string | — | Shell command to execute after each received file |
| `--quiet` | bool | false | Quiet mode — minimal output |

**Examples:**
```bash
localgo share --file document.pdf
localgo share --file document.pdf --file image.jpg
localgo share --file data.zip --pin 1234
```

---

## `localgo send`

Sends one or more files to a destination device.

**Usage:**
```bash
localgo send --file FILE --to DEVICE [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file` | string | — | File or directory to send (required, can be repeated) |
| `--to` | string | — | Target device alias (required) |
| `--port` | int | auto-detect | Target device port |
| `--timeout` | int | 30 | Send timeout in seconds |
| `--alias` | string | from config | Sender alias |

**Discovery Logic:**
1. **Multicast Burst**: Attempts to find the device via rapid Multicast (1.5s).
2. **HTTP Scan Fallback**: If not found, scans the local subnet (IPs 1–254) via HTTP/S.
3. **Transfer**: Once found, initiates the LocalSend v2 upload protocol.

**Exit Codes:**
- `0`: Success.
- `1`: File not found / Connection error / Timeout.

**Examples:**
```bash
localgo send --file document.pdf --to MyPhone
localgo send --file image.jpg --file text.txt --to MyDevice
localgo send --file data.zip --to RemotePC --timeout 60
```

---

## `localgo discover`

Passive/active discovery tool. Sends an announcement and listens for responses via multicast.

**Usage:**
```bash
localgo discover [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--timeout` | int | 5 | Discovery timeout in seconds |
| `--json` | bool | false | Output in JSON format |
| `--quiet` | bool | false | Quiet mode — only show results |

**Output:**
Returns a list of devices currently online and reachable via Multicast.
For devices that block Multicast, use `scan`.

---

## `localgo scan`

Active network scanner. Iterates through the local subnet IP range using HTTP/S requests.

**Usage:**
```bash
localgo scan [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--timeout` | int | 15 | Scan timeout in seconds |
| `--port` | int | from config | Port to scan |
| `--json` | bool | false | Output in JSON format |
| `--quiet` | bool | false | Quiet mode — only show results |

**Why use this?**
- Use this if `discover` returns nothing.
- Useful in strict corporate networks where UDP Multicast is blocked but TCP is allowed.
- Finds devices running LocalSend in "Hidden" mode (if they respond to direct IP queries).

---

## `localgo devices`

Shows all recently discovered devices on the network. Performs a short (2s) multicast scan internally.

**Usage:**
```bash
localgo devices [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Output in JSON format |

---

## `localgo info`

Prints the current configuration state.

**Usage:**
```bash
localgo info [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Output in JSON format |

**Output:**
Displays Alias, Port, Protocol, Fingerprint, and Download Directory.
Useful for verifying env vars are picked up correctly.

---

## Global Flags

These flags can be passed before any subcommand.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--verbose` | bool | false | Enable debug logging |
| `--json` | bool | false | Enable JSON log output |
| `-h`, `--help` | — | — | Show help |
| `-v`, `--version` | — | — | Show version |
