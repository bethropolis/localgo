# CLI Reference

## Global Flags

These flags can be passed before any subcommand.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--verbose` | bool | `false` | Enable debug logging |
| `--json` | bool | `false` | Enable JSON log output |
| `--no-color` | bool | `false` | Disable colored output |
| `--config` | string | — | Config file path |
| `--private`, `-p` | bool | `false` | Hide device identity (alias, model) during discovery and transfer |
| `-v`, `--version` | — | — | Show version information |
| `-h`, `--help` | — | — | Show help |

---

## `localgo serve`

Starts the receiver server. Runs in the foreground and accepts incoming file transfers and clipboard text from LocalSend-compatible devices.

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
| `--daemon`, `-d` | bool | false | Run server as a background daemon |
| `--open` | bool | false | Open download directory after transfer completes |
| `--iface` | string | — | Multicast network interface name |

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
localgo serve --daemon
localgo serve --open
```

**Behavior:**
- Starts HTTP/S server on port 53317 (or configured port).
- Joins Multicast group to listen for discovery announcements.
- Accepts upload requests; files are saved to `LOCALSEND_DOWNLOAD_DIR`.
- Incoming `text/plain` transfers are copied to the system clipboard by default (use `--no-clipboard` to save as a file instead).
- To stop, press `Ctrl+C` or use `localgo stop` when running as a daemon.

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
| `--file` | stringSlice | — | File or directory to share (can be repeated) |
| `--port` | int | from config | Port to run the server on |
| `--http` | bool | false | Deprecated (HTTP is now default for share) |
| `--https` | bool | false | Use HTTPS (browsers will reject self-signed certs) |
| `--pin` | string | — | PIN for authentication |
| `--alias` | string | from config | Device alias |
| `--auto-accept` | bool | false | Auto-accept incoming files without prompting |
| `--no-clipboard` | bool | false | Save incoming text as a file instead of copying to clipboard |
| `--history` | string | — | Path to transfer history JSONL file |
| `--exec` | string | — | Shell command to execute after each received file |
| `--quiet` | bool | false | Quiet mode — minimal output |
| `--zip` | bool | false | Zip directories before sharing |
| `--concurrency` | int | 0 | Max parallel uploads (0 = use default) |
| `--iface` | string | — | Multicast network interface name |

**Examples:**
```bash
localgo share --file document.pdf
localgo share --file document.pdf --file image.jpg
localgo share --file data.zip --pin 1234
localgo share --file mydir --zip
```

---

## `localgo send`

Sends one or more files to a destination device.

**Usage:**
```bash
localgo send --file FILE [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file` | stringSlice | — | File or directory to send (can be repeated) |
| `--to` | string | — | Target device alias (omit to pick interactively) |
| `--ip` | string | — | Target device IP (with optional `:port`, skips discovery) |
| `--port` | int | auto-detect | Target device port |
| `--timeout` | int | 30 | Send timeout in seconds |
| `--alias` | string | from config | Sender alias |
| `--concurrency` | int | 0 | Max parallel uploads (0 = use default) |
| `--iface` | string | — | Multicast network interface name |
| `--clipboard`, `-c` | bool | false | Send current system clipboard text directly |
| `--stdin` | bool | false | Send text read from standard input (stdin) |

**Discovery Logic:**
1. **Direct IP** (`--ip`): Skips discovery entirely, sends directly to the given IP:port.
2. **Multicast Burst**: Attempts to find the device via rapid Multicast (1.5s).
3. **HTTP Scan Fallback**: If not found, scans the local subnet (IPs 1–254) via HTTP/S.
4. **Transfer**: Once found, initiates the LocalSend v2 upload protocol.

**Exit Codes:**
- `0`: Success.
- `1`: File not found / Connection error / Timeout.

**Examples:**
```bash
localgo send --file document.pdf --to MyPhone
localgo send --file image.jpg --file text.txt --to MyDevice
localgo send --file data.zip --to RemotePC --timeout 60
localgo send --ip 192.168.1.100:53317 --file doc.pdf
localgo send --clipboard --to MyPhone
cat report.txt | localgo send --stdin --to MyPhone
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
| `--timeout` | int | 10 | Discovery timeout in seconds |
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
| `--range` | string | — | CIDR range to scan (e.g. `192.168.1.0/24`) |
| `--timeout` | int | 15 | Scan timeout in seconds |
| `--port` | int | from config | Port to scan |
| `--json` | bool | false | Output in JSON format |
| `--quiet` | bool | false | Quiet mode — only show results |

**Why use this?**
- Use this if `discover` returns nothing.
- Useful in strict corporate networks where UDP Multicast is blocked but TCP is allowed.
- Finds devices running LocalSend in "Hidden" mode (if they respond to direct IP queries).
- Use `--range` to scan a specific CIDR range instead of auto-detected subnets.

---

## `localgo devices`

Shows all recently discovered devices on the network. Reads from the local peer cache.

**Usage:**
```bash
localgo devices [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Output in JSON format |
| `--probe` | bool | false | Probe cached devices to verify if they are currently online |

---

## `localgo history`

Shows the file transfer history log.

**Usage:**
```bash
localgo history [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | 10 | Maximum number of entries to display |
| `--clear` | bool | false | Clear all transfer history logs |

**Examples:**
```bash
localgo history
localgo history --limit 20
localgo history --clear
```

---

## `localgo info`

Prints the current device information and configuration.

**Usage:**
```bash
localgo info [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Output in JSON format |

**Output:**
Displays Alias, Version, Device Model/Type, Fingerprint, Port, Protocol, Download Directory, PIN status, and Multicast address.
Useful for verifying env vars are picked up correctly.

---

## `localgo config`

Manage LocalGo configuration. Reads and writes the YAML config file.

**Usage:**
```bash
localgo config <subcommand> [args]
```

**Subcommands:**

### `localgo config get <key>`
Get a single config value by key.

### `localgo config set <key> <value>`
Set a config value. Automatically detects the type (int, bool, float64, string).

### `localgo config list`
List all config values.

### `localgo config path`
Show the config file path.

**Examples:**
```bash
localgo config get port
localgo config set alias "MyDevice"
localgo config list
localgo config path
```

---

## `localgo stop`

Stops a running LocalGo daemon.

**Usage:**
```bash
localgo stop
```

**Behavior:**
- Reads the PID from `localgo.pid`.
- Sends `SIGTERM` (Unix) or kills the process (Windows).
- Removes the PID file.
- Polls for graceful exit up to 5 seconds before sending `SIGKILL`.

---

## `localgo version`

Shows version information.

**Usage:**
```bash
localgo version
```

**Output:**
Displays the version, git commit, and build date.

---

## `localgo completion`

Generates shell completion scripts.

**Usage:**
```bash
localgo completion [bash|zsh|fish|powershell]
```

**Examples:**
```bash
localgo completion bash > /etc/bash_completion.d/localgo
localgo completion zsh > /usr/local/share/zsh/site-functions/_localgo
localgo completion fish > ~/.config/fish/completions/localgo.fish
```

---

## `localgo docker-start`

Sets up permissions and drops privileges before running `serve` inside a Docker container.

**Usage:**
```bash
localgo docker-start [serve flags...]
```

**Behavior:**
- Reads `PUID`/`PGID` environment variables (default 1000).
- Creates and chowns `/app/downloads` and `/app/config`.
- Drops privileges via `setgid`/`setuid` on Linux.
- Execs the binary with remaining args (forwarded directly to `serve`).

---

## `localgo health`

Runs a health check against the local server.

**Usage:**
```bash
localgo health
```

**Behavior:**
- Sends `GET` to `https://127.0.0.1:<port>/api/localsend/v2/info` with a 3-second timeout.
- Exits 0 on HTTP 200, exits 1 otherwise.
