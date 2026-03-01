# CLI Reference

## `localgo-cli serve`

Starts the receiver server. It runs in the foreground.

**Usage:**
```bash
localgo-cli serve [flags]
```

**Behavior:**
- Starts HTTP/S server on port 53317 (or configured port).
- Joins Multicast group to listen for discovery announcements.
- Accepts upload requests rooted at `LOCALSEND_DOWNLOAD_DIR`.
- To stop, press `Ctrl+C`.

---

## `localgo-cli send`

Sends a single file to a destination device.

**Usage:**
```bash
localgo-cli send --file <path> --to <alias> [flags]
```

**Discovery Logic:**
1.  **Multicast Burst**: Attempts to find the device via rapid Multicast (1.5s).
2.  **HTTP Scan Fallback**: If not found, scans the local subnet (IPs 1-254) via HTTP/S.
3.  **Transfer**: Once found, initiates the LocalSend v2 upload protocol.

**Exit Codes:**
- `0`: Success.
- `1`: File not found / Connection error / Timeout.

---

## `localgo-cli discover`

Passive/Active discovery tool. It sends an announcement and listens for responses.

**Usage:**
```bash
localgo-cli discover [flags]
```

**Output:**
Returns a list of devices currently online and reachable via Multicast.
For devices that block Multicast, use `scan`.

---

## `localgo-cli scan`

Active network scanner. It iterates through the local subnet IP range.

**Usage:**
```bash
localgo-cli scan [--port <port>] [flags]
```

**Why use this?**
- Use this if `discover` returns nothing.
- Useful in strict corporate networks where UDP Multicast is blocked but TCP is allowed.
- Finds devices running LocalSend in "Hidden" mode (if they respond to direct IP queries).

---

## `localgo-cli share`

Shares files so other devices can download them. The server announces itself with `Download: true` over multicast.

**Usage:**
```bash
localgo-cli share --file <path> [flags]
```

**Flags:**
- `--file <path>` — File to share (required, can be repeated for multiple files)
- `--port <port>` — Port to run the server on
- `--http` — Use HTTP instead of HTTPS
- `--pin <pin>` — Require PIN for incoming transfers
- `--alias <name>` — Device alias
- `--auto-accept` — Auto-accept incoming files without prompting

**Example:**
```bash
localgo-cli share --file document.pdf --file image.jpg
```

---

## `localgo-cli devices`

Shows all recently discovered devices on the network.

**Usage:**
```bash
localgo-cli devices [--json] [--quiet]
```

**Flags:**
- `--json` — Output in JSON format
- `--quiet` — Only show device aliases (no details)

---

## `localgo-cli info`

Prints the current configuration state.

**Usage:**
```bash
localgo-cli info [--json]
```

**Output:**
Displays Alias, Port, Protocol, Fingerprint, and Download Directory.
Useful for verifying env vars are picked up correctly.
