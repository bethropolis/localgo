# Docker Deployment Guide

This guide covers deploying LocalGo using Docker, Podman, and Docker Compose.

## Quick Start

### Using the Pre-built Image (Recommended)

```bash
# Pull the image from GitHub Container Registry
docker pull ghcr.io/bethropolis/localgo:latest

# Run the container
docker run -d \
  --name localgo \
  --network host \
  -v ./downloads:/app/downloads \
  -v ./config:/app/config \
  -e LOCALSEND_ALIAS="My Server" \
  ghcr.io/bethropolis/localgo:latest
```

### Using Docker Compose

```bash
# Start LocalGo with docker-compose
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the service
docker-compose down
```

## Podman

LocalGo includes a `Containerfile` for Podman builds:

```bash
# Build the image
podman build -t localgo:latest .

# Run rootless (no special setup needed)
podman run -d \
  --name localgo \
  --network host \
  -v ./downloads:/app/downloads \
  -v ./config:/app/config \
  -e LOCALSEND_ALIAS="My Server" \
  localhost/localgo:latest

# Or pull from GHCR
podman pull ghcr.io/bethropolis/localgo:latest
podman run -d --network host -v ./downloads:/app/downloads -v ./config:/app/config ghcr.io/bethropolis/localgo:latest
```

> **Note:** Podman runs rootless by default. The entrypoint script automatically detects rootless Podman and skips privilege-dropping.

## Docker Compose Configuration

The included `docker-compose.yml` provides a complete setup with:
- Volume persistence for downloads and config
- HTTP health checks
- Environment variable configuration
- PUID/PGID support for permission management

### Volume Persistence

```yaml
volumes:
  - ./downloads:/app/downloads        # Store received files
  - ./config:/app/config              # Persist TLS certificates
```

**Permissions are handled automatically!** The entrypoint script fixes ownership on startup. For manual control:

```bash
# Set ownership to match host user (UID/GID)
chown -R 1000:1000 ./downloads ./config
```

### Environment Variables

| Variable | Description | Default |
|---|---|---|
| `LOCALSEND_ALIAS` | Device name | Hostname |
| `LOCALSEND_PORT` | Port number | 53317 |
| `LOCALSEND_PIN` | PIN for authentication | (none) |
| `LOCALSEND_FORCE_HTTP` | Disable HTTPS | false |
| `LOCALSEND_DEVICE_TYPE` | Device type (mobile/desktop/web/headless/server/laptop/tablet/other) | "desktop" |
| `LOCALSEND_DEVICE_MODEL` | Device model string | "LocalGo" |
| `LOCALSEND_AUTO_ACCEPT` | Auto-accept incoming files | false |
| `LOCALSEND_NO_CLIPBOARD` | Save incoming text as a file instead of clipboard | false |
| `LOCALSEND_LOG_LEVEL` | Log level (debug/info/warn/error) | "info" |
| `PUID` | User ID for file ownership | 1000 |
| `PGID` | Group ID for file ownership | 1000 |

> **Note:** Clipboard integration is automatically disabled inside containers — no display server or clipboard tools (`xclip`, `wl-copy`, etc.) are present in the image. Incoming `text/plain` transfers are saved as `.txt` files in the download directory instead. This is equivalent to setting `LOCALSEND_NO_CLIPBOARD=true` and requires no extra configuration.

### Basic Usage

```bash
# Start in background
docker-compose up -d

# View logs
docker-compose logs -f

# Restart service
docker-compose restart

# Stop and remove
docker-compose down
```

## Manual Docker Build

```bash
# Build the image
docker build -t localgo:latest .

# Run the container
docker run -d \
  --name localgo \
  --network host \
  -v ./downloads:/app/downloads \
  -v ./config:/app/config \
  -e LOCALSEND_ALIAS="My Docker Server" \
  localgo:latest
```

### Build with Version Information

```bash
docker build \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%d_%H:%M:%S') \
  -t localgo:$(git describe --tags --always) .
```

## Network Configuration

### Linux (Recommended)

Use `network_mode: host` for full multicast support:

```yaml
services:
  localgo:
    network_mode: host
```

This allows the container to access the host's network interfaces directly, enabling multicast discovery.

### macOS/Windows

Use port mapping instead:

```yaml
services:
  localgo:
    ports:
      - "53317:53317/tcp"
      - "53317:53317/udp"
```

**Note:** Multicast discovery may not work as reliably on macOS/Windows. Use HTTP scanning (`localgo scan`) as an alternative.

## Health Checks

The Docker image includes an HTTP healthcheck that queries the `/api/localsend/v2/info` endpoint every 30 seconds.

### Check Health Status

```bash
# With docker
docker inspect localgo | jq '.[0].State.Health'

# With docker-compose
docker-compose ps
```

### Health Check Configuration

In `docker-compose.yml`:
```yaml
healthcheck:
  test: ["CMD", "wget", "-q", "--spider", "http://localhost:53317/api/localsend/v2/info"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 5s
```

## Resource Limits

The compose file includes sensible resource limits:

```yaml
deploy:
  resources:
    limits:
      cpus: '1'
      memory: 512M
    reservations:
      cpus: '0.5'
      memory: 256M
```

## Troubleshooting

### Check Container Logs

```bash
# Follow logs in real-time
docker-compose logs -f

# View last 100 lines
docker-compose logs --tail=100
```

### Test Discovery

**From host to container:**
```bash
./localgo discover --timeout 10
```

**From container:**
```bash
docker-compose exec localgo localgo info
docker-compose exec localgo localgo discover --timeout 10
```

### Permission Issues / SELinux

If you are on **Fedora, RHEL, or CentOS**, SELinux may block access to mounted volumes. Use the `:z` suffix:

```yaml
volumes:
  - ./downloads:/app/downloads:z
  - ./config:/app/config:z
```

### Network Issues

**Multicast not working:**
1. Verify network mode is set to `host` (Linux only)
2. Check firewall settings allow UDP port 53317
3. Use HTTP scanning as fallback: `docker-compose exec localgo localgo scan`

## Security Considerations

1. **PIN protection**: Always set `LOCALSEND_PIN` in production
2. **HTTPS**: Use HTTPS (default) instead of HTTP
3. **Network isolation**: Use custom networks to isolate containers
4. **Regular updates**: Keep the Docker image updated

## See Also

- [Configuration Guide](CONFIGURATION.md) - Detailed configuration options
- [Getting Started](GETTING_STARTED.md) - Initial setup guide
- [CLI Reference](CLI_REFERENCE.md) - Command-line usage
