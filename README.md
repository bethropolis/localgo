# LocalGo

A Go implementation of the LocalSend protocol for secure, cross-platform file sharing.

[![Go Version](https://img.shields.io/badge/Go-1.19+-blue.svg)](https://golang.org)
[![Protocol](https://img.shields.io/badge/Protocol-LocalSend%20v2.1-green.svg)](https://github.com/localsend/protocol)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## 🚀 Features

### Core Functionality
- ✅ **Complete LocalSend v2.1 Protocol** - Full compatibility with LocalSend ecosystem
- ✅ **Secure File Transfer** - HTTPS with self-signed certificates and PIN protection
- ✅ **Multi-Platform Discovery** - Multicast UDP + HTTP fallback for reliable device detection
- ✅ **Cross-Platform** - Works on Linux, macOS, and Windows
- ✅ **High Performance** - Efficient file transfer with progress tracking

## 📦 Quick Start

### Installation

**Option 1: One-command installation (Recommended)**
```bash
# User installation
./scripts/install.sh

# System-wide with service
sudo ./scripts/install.sh --mode system --service --create-user
```

**Option 2: Manual build**
```bash
git clone https://github.com/bethropolis/localgo.git
cd localgo
make build
```

### Basic Usage

```bash
# Start server to receive files
localgo-cli serve

# Discover devices on network
localgo-cli discover

# Send a file
localgo-cli send --file document.pdf --to "John's Phone"

# Get help
localgo-cli help
localgo-cli help send
```

## 📖 Usage Guide

### Starting the Server

```bash
# Basic server (HTTPS on port 53317)
localgo-cli serve

# Custom configuration
localgo-cli serve --port 8080 --http --alias "MyServer" --pin 123456

# With environment variables
export LOCALSEND_ALIAS="File Server"
export LOCALSEND_DOWNLOAD_DIR="/srv/files"
localgo-cli serve
```

### Sending Files

```bash
# Send to specific device
localgo-cli send --file presentation.pptx --to "MacBook Pro"

# Send with custom timeout
localgo-cli send --file large-video.mp4 --to "Desktop" --timeout 300

# Send with custom sender alias
localgo-cli send --file report.pdf --to "Office PC" --alias "Mobile Device"
```

### Discovery and Scanning

```bash
# Discover devices (multicast)
localgo-cli discover --timeout 10

# Scan network (HTTP)
localgo-cli scan --port 53317

# JSON output for scripting
localgo-cli discover --json | jq '.devices[].alias'

# Quiet mode for automation
localgo-cli scan --quiet --timeout 5
```

### Device Information

```bash
# Show device configuration
localgo-cli info

# JSON format for scripts
localgo-cli info --json

# Check version
localgo-cli version
```

## ⚙️ Configuration

### Environment Variables

```bash
# Device Configuration
LOCALSEND_ALIAS="My Device"              # Device name
LOCALSEND_PORT=53317                     # Server port
LOCALSEND_DOWNLOAD_DIR="./downloads"     # Download directory
LOCALSEND_DEVICE_TYPE="desktop"          # Device type

# Network Configuration
LOCALSEND_MULTICAST_GROUP="224.0.0.167" # Multicast address
LOCALSEND_FORCE_HTTP=false               # Use HTTP instead of HTTPS

# Security Configuration
LOCALSEND_PIN="123456"                   # PIN for authentication
LOCALSEND_SECURITY_DIR="./.localgo_security" # Security files location

# Logging Configuration
LOCALSEND_LOG_LEVEL="info"               # Log level (debug,info,warn,error)
LOCALSEND_VERBOSE=false                  # Verbose output
LOCALSEND_NO_COLOR=false                 # Disable colored output
```

### Configuration File

Create `localgo.env`:
```bash
# Copy example configuration
cp scripts/localgo.env.example localgo.env

# Edit configuration
editor localgo.env

# Use configuration
source localgo.env && localgo-cli serve
```

### Command-Line Flags

Each command supports specific flags:

```bash
# Serve command
localgo-cli serve --port 8080 --http --pin 123456 --alias "Server" --dir "/tmp" --verbose

# Send command
localgo-cli send --file data.zip --to "Device" --port 8080 --timeout 60 --alias "Sender"

# Discovery commands
localgo-cli discover --timeout 10 --json --quiet
localgo-cli scan --port 8080 --timeout 15 --json
```

## 🔧 System Service

### Installation

```bash
# Install as system service
sudo ./scripts/install.sh --mode system --service --create-user
```

### Service Management

```bash
# Enable and start service
sudo systemctl enable localgo
sudo systemctl start localgo

# Check status
sudo systemctl status localgo

# View logs
sudo journalctl -u localgo -f

# Restart service
sudo systemctl restart localgo
```

### Service Configuration

Edit `/etc/localgo/localgo.env`:
```bash
LOCALSEND_ALIAS="File Server"
LOCALSEND_PORT=53317
LOCALSEND_DOWNLOAD_DIR="/srv/localgo/downloads"
LOCALSEND_PIN="secure123"
LOCALSEND_DEVICE_TYPE="server"
```

## 🤖 Automation & Scripting

### JSON Output

Perfect for integration with other tools:

```bash
# Get device list
DEVICES=$(localgo-cli scan --json --timeout 5)
echo "$DEVICES" | jq -r '.devices[].alias'

# Check if service is running
localgo-cli info --json | jq -r '.alias + " on port " + (.port|tostring)'

# Monitor file transfers
localgo-cli info --json | jq '.downloadDir'
```

### Batch Operations

```bash
# Send multiple files
find /uploads -name "*.pdf" | while read file; do
    localgo-cli send --file "$file" --to "PrintServer"
done

# Health check script
#!/bin/bash
if localgo-cli info --json >/dev/null 2>&1; then
    echo "LocalGo is healthy"
    exit 0
else
    echo "LocalGo is not responding"
    exit 1
fi
```

### Docker Integration

```dockerfile
FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY . .
RUN make build

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/localgo-cli .
COPY scripts/localgo.env.example localgo.env
CMD ["./localgo-cli", "serve"]
```

## 💻 Development

### Building

```bash
# Build binary
make build

# Run tests
make test

# Run with coverage
make test-coverage

# Clean build artifacts
make clean
```

### Project Structure

```
localgo/
├── cmd/localgo-cli/           # CLI application entry point
├── pkg/                       # Core library packages
│   ├── cli/                  # CLI output utilities
│   ├── config/               # Configuration management
│   ├── crypto/               # TLS certificates and fingerprints
│   ├── discovery/            # Network discovery (multicast + HTTP)
│   ├── httputil/             # HTTP response utilities
│   ├── logging/              # Structured logging
│   ├── model/                # Data structures (Device, File, DTOs)
│   ├── network/              # Network interface utilities
│   ├── send/                 # File sending logic
│   ├── server/               # HTTP server and handlers
│   └── storage/              # File storage management
├── scripts/                  # Installation and utility scripts
├── protocol/                 # LocalSend protocol specification
└── downloads/                # Default download directory
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request

## 🌟 Examples

### Home Media Server

```bash
# Configure as media server
export LOCALSEND_ALIAS="Home Media Server"
export LOCALSEND_DOWNLOAD_DIR="/media/shared"
export LOCALSEND_DEVICE_TYPE="server"
export LOCALSEND_PIN="family123"

# Install as system service
sudo ./scripts/install.sh --mode system --service --create-user

# Start service
sudo systemctl start localgo
```

### Development Setup

```bash
# Configure for development
export LOCALSEND_ALIAS="Dev-$(whoami)"
export LOCALSEND_PORT=8080
export LOCALSEND_FORCE_HTTP=true
export LOCALSEND_VERBOSE=true
export LOCALSEND_LOG_LEVEL="debug"

# Start with verbose logging
localgo-cli serve --verbose
```

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Test file transfer
  run: |
    # Start receiver
    localgo-cli serve --http --port 8080 &
    sleep 2

    # Send test file
    echo "test data" > test.txt
    localgo-cli send --file test.txt --to "$(localgo-cli info --json | jq -r '.alias')"

    # Verify transfer
    test -f downloads/test.txt
```

### Network Monitoring

```bash
#!/bin/bash
# Monitor LocalGo devices on network

while true; do
    echo "=== LocalGo Network Scan $(date) ==="
    localgo-cli scan --json --timeout 10 | jq -r '
        .devices[] |
        "\(.alias) (\(.deviceType)) - \(.ip):\(.port) - \(.protocol)"
    '
    echo
    sleep 60
done
```

## 🛠️ Troubleshooting

### Common Issues

**Port already in use:**
```bash
# Check what's using the port
sudo netstat -tlnp | grep 53317

# Use different port
localgo-cli serve --port 8080
```

**Discovery not working:**
```bash
# Check firewall
sudo ufw status

# Test network connectivity
localgo-cli scan --timeout 10

# Use HTTP discovery
localgo-cli scan --json
```

**Permission denied:**
```bash
# Fix download directory permissions
sudo chown -R $USER:$USER ~/Downloads/LocalGo

# Check service user permissions (systemd)
sudo journalctl -u localgo -n 50
```

### Debug Mode

```bash
# Enable verbose logging
localgo-cli serve --verbose

# Debug network issues
export LOCALSEND_LOG_LEVEL="debug"
localgo-cli discover
```

## 📋 Protocol Compliance

LocalGo implements the complete LocalSend v2.1 protocol:

- ✅ **Discovery API** - `/api/localsend/v2/register`, `/api/localsend/v2/info`
- ✅ **Upload API** - `/api/localsend/v2/prepare-upload`, `/api/localsend/v2/upload`
- ✅ **Download API** - `/api/localsend/v2/prepare-download`, `/api/localsend/v2/download`
- ✅ **Session Management** - Proper session handling with tokens and timeouts
- ✅ **Security** - TLS encryption, fingerprint validation, PIN protection
- ✅ **Discovery** - Multicast UDP announcements with HTTP fallback

## 📜 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [LocalSend Project](https://github.com/localsend/localsend) - For the excellent protocol specification
- [LocalSend Protocol](https://github.com/localsend/protocol) - For detailed protocol documentation

## 🔗 Related Projects

- [LocalSend](https://github.com/localsend/localsend) - Original Flutter implementation
- [LocalSend_rs](https://github.com/notjedi/localsend-rs) - A cli Rust implementation

---
thank you
