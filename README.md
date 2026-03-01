# LocalGo

A Go implementation of the LocalSend v2.1 protocol for secure, cross-platform file sharing.

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![Protocol](https://img.shields.io/badge/Protocol-LocalSend%20v2.1-green.svg)](https://github.com/localsend/protocol)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Features

- **Complete LocalSend v2.1 Protocol** - Works with LocalSend apps
- **Secure** - HTTPS with certificates, optional PIN protection
- **Fast Discovery** - Multicast UDP + HTTP fallback
- **Multi-file Transfers** - Send multiple files concurrently
- **Web Share** - Share files via browser download link
- **Metadata Preserved** - File timestamps preserved on transfer
- **Cross-Platform** - Linux, macOS, Windows

## Quick Start

### Installation

```bash
# User installation (recommended)
./scripts/install.sh

# System-wide with systemd
sudo ./scripts/install.sh --mode system --service --create-user
```

### Usage

```bash
# Start server to receive files
localgo-cli serve

# Discover devices
localgo-cli discover

# Send a file
localgo-cli send --file document.pdf --to "My Phone"

# Share files for web download
localgo-cli share --file document.pdf
```

### Docker

```bash
# Start with Docker Compose
docker-compose up -d

# Or build and run manually
docker build -t localgo .
docker run -d --network host -v $(pwd)/downloads:/app/downloads localgo
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOCALSEND_ALIAS` | hostname | Device name |
| `LOCALSEND_PORT` | 53317 | Server port |
| `LOCALSEND_DOWNLOAD_DIR` | ./downloads | Download directory |
| `LOCALSEND_PIN` | - | Optional PIN protection |
| `LOCALSEND_FORCE_HTTP` | false | Use HTTP instead of HTTPS |
| `LOCALSEND_DEVICE_TYPE` | desktop | Device type (mobile, desktop, server, etc.) |

### Example

```bash
export LOCALSEND_ALIAS="File Server"
export LOCALSEND_DOWNLOAD_DIR="/srv/files"
export LOCALSEND_PIN="123456"
localgo-cli serve
```

## Commands

| Command | Description |
|---------|-------------|
| `serve` | Start server to receive files |
| `share` | Share files via web download |
| `discover` | Find devices via multicast |
| `scan` | Find devices via HTTP scan |
| `send` | Send files to a device |
| `info` | Show device information |
| `devices` | List discovered devices |

Run `localgo-cli help` for more options.

## Documentation

Detailed guides available in [docs/](docs/):

- [Configuration](docs/CONFIGURATION.md) - All settings
- [Docker](docs/DOCKER.md) - Container deployment
- [CLI Reference](docs/CLI_REFERENCE.md) - Command details
- [Deployment](docs/DEPLOYMENT.md) - Production setup

## Troubleshooting

**Discovery not working:**
```bash
# Use HTTP scan instead
localgo-cli scan --timeout 15
```

**Port in use:**
```bash
# Use different port
localgo-cli serve --port 8080
```

**Permission denied:**
```bash
# Fix download directory
chmod 755 ~/Downloads/LocalGo
```

## License

MIT License - see [LICENSE](LICENSE) file.
