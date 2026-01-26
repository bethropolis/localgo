# Getting Started with LocalGo

LocalGo is a high-performance, cross-platform implementation of the LocalSend protocol. This guide will help you get up and running quickly.

## 📥 Installation

### Option 1: Pre-built Binaries (Recommended)
Download the latest release for your platform from the [Releases page](https://github.com/bethropolis/localgo/releases).

**Linux/macOS:**
```bash
tar -xzf localgo_Linux_x86_64.tar.gz
sudo mv localgo-cli /usr/local/bin/
```

**Windows:**
Extract the zip file and add the folder to your `PATH`.

### Option 2: Build from Source
Requirements: Go 1.19+

```bash
git clone https://github.com/bethropolis/localgo.git
cd localgo
make build
```

### Option 3: Install via Go
```bash
go install github.com/bethropolis/localgo/cmd/localgo-cli@latest
```

---

## 🚀 Quick Start

### 1. Receive Files
To start receiving files, simply run the server. It will automatically discover your network interface and start listening.

```bash
localgo-cli serve
```
*You are now visible to other LocalSend devices as "LocalGo".*

### 2. Send Files
To send a file, you first need to know the alias (name) of the recipient.

**Discover nearby devices:**
```bash
localgo-cli discover
```

**Send the file:**
```bash
localgo-cli send --file photos.zip --to "QuickShare"
```

---

## 💡 Common Scenarios

### Running on a Headless Server
If you are running LocalGo on a VPS or a Raspberry Pi without a monitor:

1.  **Use specific IP binding**: If you have multiple interfaces (VPN, Docker), bind to the physical LAN IP.
    *(Currently `serve` listens on `0.0.0.0`, but you can filter discovery by subnet)*
2.  **Enable Quiet Mode**: To avoid cluttering logs.
    ```bash
    localgo-cli serve --quiet &
    ```
3.  **Run as a Service**: See [Deployment Guide](DEPLOYMENT.md).

### Using in a Script
LocalGo is JSON-friendly.

```bash
# Check if "MyPhone" is online
if localgo-cli scan --json | grep -q "MyPhone"; then
    echo "Phone is online!"
fi
```

## 🔒 Security

- **Encryption**: All transfers are encrypted using TLS 1.2+ with on-the-fly generated certificates.
- **PIN Protection**: Enforce a PIN for incoming transfers.
    ```bash
    localgo-cli serve --pin 12345
    ```
    *The sender will be prompted to enter this PIN.*

## ⏭ Next Steps

- [Configuration Guide](CONFIGURATION.md) - Deep dive into flags and env vars.
- [CLI Reference](CLI_REFERENCE.md) - Full command documentation.
- [Library Guide](LIBRARY_GUIDE.md) - Embed LocalGo in your own Go apps.
- [Code Walkthrough](CODE_WALKTHROUGH.md) - Step-by-step guide to understand the codebase.

