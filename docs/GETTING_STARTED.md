# Getting Started with LocalGo

LocalGo is a high-performance, cross-platform implementation of the LocalSend protocol. This guide will help you get up and running quickly.

## Installation

### Option 1: Pre-built Binaries
Download the latest release for your platform from the [Releases page](https://github.com/bethropolis/localgo/releases).

**Linux/macOS:**
```bash
tar -xzf localgo_Linux_x86_64.tar.gz
sudo mv localgo /usr/local/bin/
```

### Option 2: Install via the install script
```bash
# User installation (installs to ~/.local/bin)
./scripts/install.sh

# System-wide with systemd service
sudo ./scripts/install.sh --mode system --service --create-user
```

### Option 3: Build from Source
Requirements: Go 1.19+

```bash
git clone https://github.com/bethropolis/localgo.git
cd localgo
make build
```

### Option 4: Install via Go
```bash
go install github.com/bethropolis/localgo/cmd/localgo@latest
```

### Option 5: Install via Homebrew (macOS/Linux)
```bash
brew install bethropolis/localgo/localgo
```

### Option 6: Install via Docker/podman
```bash
podman run -d \
  --name localgo \
  --network host \
  -v $(pwd)/downloads:/app/downloads:z \
  -v $(pwd)/config:/app/config:z \
  -e LOCALSEND_ALIAS="My Server" \
  ghcr.io/bethropolis/localgo:latest
```
> ensure the mounted `downloads` and `config` directories exist and have the correct permissions.
> for more information see [container documentation](CONTAINER.md)


---

## Quick Start

### 1. Receive Files
Start the server. It will automatically discover your network interface and start listening for incoming transfers.

```bash
localgo serve
```

You are now visible to other LocalSend devices on your network.

### 2. Discover Devices
```bash
localgo discover
```

If multicast is blocked on your network, use the HTTP scanner instead:
```bash
localgo scan
```

Or list the most recently seen devices (fast 2s scan):
```bash
localgo devices
```

### 3. Send Files
```bash
localgo send --file photos.zip --to "QuickShare"
```

You can send multiple files at once:
```bash
localgo send --file image.jpg --file document.pdf --to "MyPhone"
```

### 4. Share Files for Download
Start a share server so other devices can pull files from you:
```bash
localgo share --file document.pdf
```

Share multiple files, protected by a PIN:
```bash
localgo share --file report.pdf --file data.csv --pin 1234
```

### 5. Show Device Info
Check your current configuration (alias, fingerprint, port, etc.):
```bash
localgo info
```

---

## Common Scenarios

### Running on a Headless Server
If you are running LocalGo on a VPS or a Raspberry Pi without a display:

1. **Enable Quiet Mode** to avoid cluttering logs.
   ```bash
   localgo serve --quiet &
   ```
2. **Disable clipboard** since there is no display server — incoming text will be saved as a `.txt` file.
   ```bash
   localgo serve --no-clipboard
   ```
3. **Run as a Service**: See the [Deployment Guide](DEPLOYMENT.md).

### Using in a Script
LocalGo is JSON-friendly — pipe output into `jq` or `grep` for automation:

```bash
# Check if "MyPhone" is online
if localgo scan --json | grep -q "MyPhone"; then
    echo "Phone is online!"
fi

# Get device list as JSON
localgo devices --json | jq '.[].alias'
```

### Auto-Accept Mode
For unattended setups where you trust senders on the network:

```bash
localgo serve --auto-accept --quiet
```

Or set it permanently via an environment variable:
```bash
export LOCALSEND_AUTO_ACCEPT=true
localgo serve --quiet
```

## Security

- **Encryption**: All transfers are encrypted using TLS 1.2+ with on-the-fly generated certificates.
- **PIN Protection**: Enforce a PIN for incoming transfers.
  ```bash
  localgo serve --pin 12345
  ```
  The sender will be prompted to enter this PIN.

---

## Next Steps

- [Configuration Guide](CONFIGURATION.md) - Deep dive into all flags and env vars.
- [CLI Reference](CLI_REFERENCE.md) - Full command and flag documentation.
- [Library Guide](LIBRARY_GUIDE.md) - Embed LocalGo in your own Go apps.
- [Code Walkthrough](CODE_WALKTHROUGH.md) - Step-by-step guide to the codebase.
