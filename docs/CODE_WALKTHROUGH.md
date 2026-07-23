# Codebase Walkthrough

This document provides a deep dive into the LocalGo codebase, explaining "how it works" and the purpose of each package and file.

## Project Structure

### `cmd/localgo/`
The entry point for the application.

- **`main.go`**: The command-line interface (CLI) driver.
    - Defines `Version`, `GitCommit`, `BuildDate` ldflags vars.
    - Calls `SetVersionInfo()` to wire version info into the help system.
    - Handles signal interrupts (Ctrl+C) for graceful shutdown.
- **`main_test.go`**: Integration tests for the CLI commands.
- **`cmd/`**: Subcommand implementations (see [CLI Reference](CLI_REFERENCE.md)).

### `pkg/`
The core logic libraries.

#### `pkg/config/`
Handles application configuration.
- **`config.go`**: Defines the `Config` struct.
    - `LoadConfig()`: Loads settings from environment variables and defaults.
    - Manages the "Security Context" (TLS certificates).
    - Generates separate `RegisterDto` (discovery) and `InfoDto` (server info) structures.
    - `ProtocolVersion` constant set to `"2.0"`.
- **`viper.go`**: Initializes Viper for YAML config file support and environment variable binding.
- **`dto.go`**: DTO conversion methods (`ToMulticastDto`, `ToRegisterDto`, `ToInfoDto`).

#### `pkg/server/`
The HTTP/S server that listens for incoming files and discovery requests.
- **`server.go`**: Initializes the `http.Server` and router. Configures API routes (`/api/localsend/v2/...`).
- **`handlers/`**:
    - **`discovery_handlers.go`**: Handles `/register` (peers announcing themselves) and `/info` (returning our device info).
    - **`receive_handlers.go`**: Handles file upload requests. `PrepareUpload` validates PIN, checks disk space, returns a session token. `Upload` accepts the file stream and saves it.
    - **`receive_upload.go`**: Upload session management and file writing logic.
    - **`download_handlers.go`**: Handles file download requests (share mode).
    - **`exec.go`**: Post-receive exec hook runner.
    - **`prompt.go`**: Interactive TUI prompts for incoming transfers.
    - **`history_log.go`**: Transfer history logging.

#### `pkg/discovery/`
Implements the logic to find other LocalSend devices.
- **`service.go`**: The high-level coordinator. Starts both Multicast listening and periodic announcements.
- **`multicast.go`**: Handles UDP Multicast packets on `224.0.0.167:53317`. On announcement, sends HTTP `POST /register` response; falls back to UDP unicast.
- **`http_discovery.go`**: The "Smart Scanner". Iterates through target IPs and sends `POST /api/localsend/v2/register` to find active devices.
- **`peer_cache.go`**: Persistent peer cache for recently discovered devices.

#### `pkg/network/`
Low-level networking utilities.
- **`interfaces.go`**: `GetLocalIPAddresses`, `GetSubnetIPs`, `ParseCIDRRange`.

#### `pkg/send/`
Client-side logic for sending files.
- **`send.go`**: Discovery phase (multicast burst → HTTP subnet scan), prepare phase (metadata exchange), transfer phase (file streaming). Exports `SendToDevice()` for direct IP-based send.
- **`verify.go`**: TLS certificate fingerprint verification (MitM prevention).

#### `pkg/model/`
Go struct definitions that map to the LocalSend JSON protocol.
- **`device.go`**: Represents a peer device (Alias, IP, DeviceType, Fingerprint).
- **`dto.go`**: Data Transfer Objects for the API (e.g., `PrepareUploadRequestDto`).

#### `pkg/crypto/`
Security primitives.
- **`crypto.go`**: Generates self-signed X.509 certificates for TLS and computes the SHA-256 fingerprint of the certificate.

#### `pkg/storage/`
File storage utilities.
- **`storage.go`**: `SaveStreamToFileWithMetadata` for atomic file writes with SHA-256 verification, timestamp preservation, and progress reporting.
- **`storage_unix.go`**: `CheckFreeSpace` via `unix.Statfs` for disk space guard.

#### `pkg/metadata/`
Metadata stripping for private mode.
- **`strip.go`**: Pure stdlib JPEG EXIF (APP1/APP13 marker skipping) and PNG text chunk (tEXt/zTXt/iTXt) stripping.

#### `pkg/cli/`
CLI output utilities.
- **`output.go`**: Styled output, `AnonymizedAlias()`, `AnonymizeString()`, `PickDevice()` interactive device picker.
- **`filepicker.go`**: Interactive TUI file picker.

#### `pkg/clipboard/`
Cross-platform clipboard reading.
- **`clipboard.go`**: Reads clipboard via CLI tools (pbpaste, wl-paste, xclip, xsel, Get-Clipboard) — CGo-free.

#### `pkg/help/`
Help text and version display.
- **`help.go`**: Command help blocks and version output.

---

## Lifecycle Flows

### 1. Starting the Server (`serve`)
1.  `main.go` loads `Config` (generating certs if needed).
2.  Initializes `discovery.Service` which opens a UDP Multicast listener.
3.  Initializes `server.Server` which opens a TCP listener (HTTP/S).
4.  The server runs indefinitely, accepting files and replying to discovery probes.

### 2. Sending a File (`send`)
1.  **Discovery**:
    - `send.go` broadcasts "I am here" via Multicast.
    - Simultaneously listens for the target's response.
    - If silence after 1.5s, it calculates `GetSubnetIPs()` and probes every IP on the LAN via HTTP.
2.  **Handshake**:
    - Once IP is found, sends `POST /prepare-upload`.
    - Target validates PIN and returns a `token` and `sessionId`.
3.  **Transfer**:
    - Sender POSTs file data to `/upload` using the `sessionId`.
    - Receiver writes data to `LOCALSEND_DOWNLOAD_DIR`.

### 3. Cross-Platform Compatibility
- **Windows**: The code handles Windows-specific path separators and `.exe` extensions (in tests).
- **Linux/macOS**: Uses standard POSIX paths.
- **Discovery**: Explicitly supports both UDP (LocalSend default) and HTTP (Network firewall safe) discovery methods to ensure it works with official clients.
