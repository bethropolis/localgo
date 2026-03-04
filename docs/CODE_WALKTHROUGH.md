# Codebase Walkthrough

This document provides a deep dive into the LocalGo codebase, explaining "how it works" and the purpose of each package and file.

## 📂 Project Structure

### `cmd/localgo/`
The entry point for the application.

- **`main.go`**: The command-line interface (CLI) driver.
    - Sets up the `Application` struct.
    - Defines subcommands: `serve`, `send`, `discover`, `scan`.
    - Wires together the `config`, `server`, and `discovery` components.
    - Handles signal interrupts (Ctrl+C) for graceful shutdown.
- **`main_test.go`**: Integration tests for the CLI commands.

### `pkg/`
The core logic libraries.

#### `pkg/config/`
Handles application configuration.
- **`config.go`**: Defines the `Config` struct.
    - `LoadConfig()`: Loads settings from environment variables and defaults.
    - Manages the "Security Context" (TLS certificates).
    - Generates separate `RegisterDto` (discovery) and `InfoDto` (server info) structures.

#### `pkg/server/`
The HTTP/S server that listens for incoming files and discovery requests.
- **`server.go`**: initializes the `http.Server` and Gorilla Mux router.
    - Configures API routes (`/api/localsend/v2/...`).
- **`handlers/`**:
    - **`discovery.go`**: Handles `/register` (peers announcing themselves) and `/info` (returning our device info).
    - **`receive.go`**: Handles file upload requests.
        - `PrepareUpload`: Validates PIN, checks disk space, returns a session token.
        - `Upload`: Accepts the file stream and saves it to the download directory.
- **`services/`**: logic separate from HTTP transport.
    - **`receive_service.go`**: Manages active upload sessions and tokens.

#### `pkg/discovery/`
Implements the logic to find other LocalSend devices.
- **`service.go`**: The high-level coordinator. It starts both Multicast listening and periodic announcements.
- **`multicast.go`**: Handles UDP Multicast packets.
    - Listens on `224.0.0.167:53317`.
    - When an announcement is received, it triggers a "Response".
    - **Key Logic**: It first tries to send a response via HTTP (`POST /register`). if that fails, it falls back to a UDP unicast response.
- **`http_discovery.go`**: The "Smart Scanner".
    - Used when Multicast fails.
    - Iterates through target IP addresses (subnet scan) and sends `POST /api/localsend/v2/register` to checking for active devices.

#### `pkg/network/`
Low-level networking utilities.
- **`interfaces.go`**:
    - `GetLocalIPAddresses`: Finds all valid non-loopback interface IPs.
    - `GetSubnetIPs`: The logic that powers "Smart Scan". It takes a local IP (e.g., `192.168.1.5`) and generates the full `/24` range (`.1` to `.254`) to ensure we find all neighbors.

#### `pkg/send/`
The client-side logic for sending files.
- **`send.go`**:
    - **Discovery Phase**: First attempts a quick Multicast burst (1.5s). If no target found, triggers a full HTTP subnet scan.
    - **Prepare Phase**: Sends metadata (name, size, type) to the target.
    - **Transfer Phase**: Streams the file binary data to the target's `/upload` endpoint.

#### `pkg/model/`
Go struct definitions that map to the LocalSend JSON protocol.
- **`device.go`**: Represents a peer device (Alias, IP, DeviceType).
- **`dto.go`**: Data Transfer Objects for the API (e.g., `PrepareUploadRequestDto`).

#### `pkg/crypto/`
Security primitives.
- **`cert.go`**: Generates self-signed X.509 certificates for TLS.
- **`hash.go`**: Computes the SHA-256 fingerprint of the certificate (identity string).

---

## 🔄 Lifecycle Flows

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
