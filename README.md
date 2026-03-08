# LocalGo

[![Go Version](https://img.shields.io/badge/Go-1.24+--blue.svg)](https://golang.org)
[![Protocol](https://img.shields.io/badge/Protocol-LocalSend%20v2.1-green.svg)](https://github.com/localsend/protocol)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Build](https://github.com/bethropolis/localgo/actions/workflows/docker.yml/badge.svg)](https://github.com/bethropolis/localgo/actions/workflows/docker.yml)
[![Docker Image](https://img.shields.io/docker/pulls/ghcr.io/bethropolis/localgo.svg)](https://github.com/bethropolis/localgo/pkgs/container/localgo)
[![Release](https://img.shields.io/github/v/release/bethropolis/localgo)](https://github.com/bethropolis/localgo/releases/latest)
[![Platforms](https://img.shields.io/badge/Platforms-linux%20%7C%20macos%20%7C%20windows-informational)](https://github.com/bethropolis/localgo)

A Go implementation of the LocalSend v2.1 protocol for secure, cross-platform file sharing.

## Features

- **Complete LocalSend v2.1 Protocol** - Works with LocalSend apps
- **Secure** - HTTPS with certificates, optional PIN protection
- **Fast Discovery** - Multicast UDP + HTTP fallback
- **Multi-file Transfers** - Send multiple files concurrently
- **Web Share** - Share files via browser download link
- **Clipboard Integration** - Incoming text/plain transfers copied to clipboard automatically
- **Metadata Preserved** - File timestamps preserved on transfer
- **Cross-Platform** - Linux, macOS, Windows

## Quick Start

### Installation

####  User installation (recommended)
```bash
# clone repo
git clone https://github.com/bethropolis/localgo.git
cd localgo

# installs a user systemd service and completions
./scripts/install.sh
```

#### using go (no service and completions)
```bash
go install github.com/bethropolis/localgo/cmd/localgo@latest
```

#### using homebrew
```bash
brew tap bethropolis/tap
brew install localgo
```

> [!NOTE]
> more install options in [installation documentation](docs/GETTING_STARTED.md)

### Usage

```bash
# Start server to receive files
localgo serve

# Discover devices
localgo discover

# Send a file
localgo send --file document.pdf --to "My Phone"

# Share files for web download
localgo share --file document.pdf
```

### Docker and Podman

Read the [container documentation](docs/CONTAINER.md) for more information.

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOCALSEND_ALIAS` | hostname | Device name |
| `LOCALSEND_PORT` | 53317 | Server port |
| `LOCALSEND_DOWNLOAD_DIR` | ./downloads | Download directory |
| `LOCALSEND_PIN` | — | Optional PIN protection |
| `LOCALSEND_FORCE_HTTP` | false | Use HTTP instead of HTTPS |
| `LOCALSEND_DEVICE_TYPE` | desktop | Device type (mobile/desktop/laptop/tablet/server/headless/web/other) |
| `LOCALSEND_DEVICE_MODEL` | LocalGo | Device model string |
| `LOCALSEND_AUTO_ACCEPT` | false | Auto-accept incoming files without prompting |
| `LOCALSEND_NO_CLIPBOARD` | false | Save incoming text as a file instead of clipboard |
| `LOCALSEND_LOG_LEVEL` | info | Log verbosity (debug/info/warn/error) |

### Example

```bash
export LOCALSEND_ALIAS="File Server"
export LOCALSEND_DOWNLOAD_DIR="/srv/files"
export LOCALSEND_PIN="123456"
localgo serve
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

Run `localgo help` for more options.

## Documentation

Detailed guides available in [docs/](docs/):

- [Configuration](docs/CONFIGURATION.md) - All settings
- [Container](docs/CONTAINER.md) - Container deployment
- [CLI Reference](docs/CLI_REFERENCE.md) - Command details
- [Deployment](docs/DEPLOYMENT.md) - Production setup

## Troubleshooting

**Discovery not working:**
```bash
# Use HTTP scan instead
localgo scan --timeout 15
```

**Port in use:**
```bash
# Use different port
localgo serve --port 8080
```

**Permission denied:**
```bash
# Fix download directory
chmod 755 ~/Downloads/LocalGo
```

## License

MIT License - see [LICENSE](LICENSE) file.
