# Docker Deployment Guide

This guide covers deploying LocalGo using Docker and Docker Compose.

## Quick Start

```bash
# Start LocalGo with docker-compose
docker-compose up -d

# View logs
docker-compose logs -f

# Check status
docker-compose ps

# Stop the service
docker-compose down
```

## Docker Compose Configuration

The included `docker-compose.yml` provides a complete setup with:
- Volume persistence for downloads and config
- Health checks
- Environment variable configuration
- Resource limits

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

### Volume Persistence

**Important:** Volumes persist data across container restarts:

```yaml
volumes:
  - ./downloads:/app/downloads        # Store received files
  - ./config:/app/config              # Persist TLS certificates
```

**Permissions are handled automatically!** The container's entrypoint script automatically fixes ownership and permissions when the container starts, so you don't need to manually run `chown` commands.

**Why volume persistence matters:**
- **Consistent fingerprint** - Certificate persists across restarts
- **File preservation** - Downloaded files survive container updates
- **Easy backup** - Just backup the host directories

### Environment Variables

Configure LocalGo using environment variables in `docker-compose.yml`:

```yaml
environment:
  - LOCALSEND_ALIAS=LocalGo-Docker
  - LOCALSEND_PORT=53317
  - LOCALSEND_PIN=your-pin-here
  - LOCALSEND_FORCE_HTTP=false
  - LOCALSEND_LOG_LEVEL=info
```

Or create a `.env` file in the project root:

```bash
LOCALSEND_ALIAS=My Server
LOCALSEND_PORT=53317
LOCALSEND_PIN=123456
LOCALSEND_DEVICE_TYPE=server
LOCALSEND_LOG_LEVEL=info
```

Then start with:
```bash
docker-compose up -d
```

## Manual Docker Build

### Basic Build

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

**Note:** Multicast discovery may not work as reliably on macOS/Windows. Use HTTP scanning (`localgo-cli scan`) as an alternative.

## Health Checks

The Docker image includes built-in health checks that run `localgo-cli info` every 30 seconds.

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
  test: ["CMD", "localgo-cli", "info"]
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

Adjust these based on your needs and available resources.

## Troubleshooting

### Check Container Logs

```bash
# Follow logs in real-time
docker-compose logs -f

# View last 100 lines
docker-compose logs --tail=100

# View logs for specific service
docker-compose logs localgo
```

### Test Discovery

**From host to container:**
```bash
# Install localgo-cli on host
./localgo-cli discover --timeout 10
```

**From container:**
```bash
# Get device info
docker-compose exec localgo localgo-cli info

# Test discovery from inside
docker-compose exec localgo localgo-cli discover --timeout 10
```

### Permission Issues / SELinux

**Common Error:** `permission denied` even after fixing ownership

If you are on **Fedora, RHEL, or CentOS**, SELinux may block access to mounted volumes.

**Solution:** Use the `:z` suffix on volume mounts in `docker-compose.yml`:

```yaml
volumes:
  - ./downloads:/app/downloads:z
  - ./config:/app/config:z
```

The `:z` tells Docker to relabel the directory content so it can be shared with the container.

**Standard Permission Fix:**

If SELinux is not the issue (e.g. Ubuntu/Debian), fix ownership:

```bash
# Fix permissions - container runs as UID 1000
sudo chown -R 1000:1000 ./downloads ./config
chmod -R 755 ./downloads ./config
```

**Check container logs for permission errors:**
```bash
docker-compose logs | grep -i "permission denied"
```

**Verify from inside container:**
```bash
# Get shell access
docker-compose exec localgo sh

# Check who you are
id

# Test write permission
touch /app/downloads/test.txt
rm /app/downloads/test.txt
```

### Network Issues

**Multicast not working:**
1. Verify network mode is set to `host` (Linux only)
2. Check firewall settings allow UDP port 53317
3. Use HTTP scanning as fallback: `docker-compose exec localgo localgo-cli scan`

**Cannot access from other devices:**
1. Check port mappings in docker-compose.yml
2. Verify firewall allows TCP/UDP port 53317
3. Ensure devices are on the same network

### Inspect Container

```bash
# Get shell access
docker-compose exec localgo sh

# View container details
docker inspect localgo

# Check network settings
docker network inspect bridge  # or host
```

## Advanced Configuration

### Custom Network

If you need a custom network instead of host mode:

```yaml
services:
  localgo:
    networks:
      - localgo-net

networks:
  localgo-net:
    driver: bridge
```

### Multiple Instances

Run multiple LocalGo instances with different configurations:

```yaml
# docker-compose.override.yml
services:
  localgo-1:
    extends:
      service: localgo
    environment:
      - LOCALSEND_ALIAS=LocalGo-1
      - LOCALSEND_PORT=53317
    volumes:
      - ./downloads-1:/app/downloads
      - ./config-1:/app/config
  
  localgo-2:
    extends:
      service: localgo
    environment:
      - LOCALSEND_ALIAS=LocalGo-2
      - LOCALSEND_PORT=53318
    volumes:
      - ./downloads-2:/app/downloads
      - ./config-2:/app/config
```


## Performance Tips

1. **Use volume mounts** for better I/O performance vs bind mounts
2. **Limit resource usage** to prevent container from consuming all resources
3. **Use host network** on Linux for better multicast performance
4. **Keep images updated** for security patches

## Security Considerations

1. **PIN protection**: Always set `LOCALSEND_PIN` in production
2. **HTTPS**: Use HTTPS (default) instead of HTTP
3. **Network isolation**: Use custom networks to isolate containers
4. **Volume permissions**: Set appropriate permissions on mounted volumes
5. **Regular updates**: Keep the Docker image updated

## See Also

- [Configuration Guide](CONFIGURATION.md) - Detailed configuration options
- [Getting Started](GETTING_STARTED.md) - Initial setup guide
- [CLI Reference](CLI_REFERENCE.md) - Command-line usage
