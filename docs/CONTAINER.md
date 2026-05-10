# Docker & Podman Deployment Guide

LocalGo supports Docker and Podman with two image variants:

| Variant | Image tag | Base | Size | Includes shell |
|---------|-----------|------|------|----------------|
| **Standard** | `ghcr.io/bethropolis/localgo:latest` | Alpine | ~25 MB | Yes |
| **Scratch** | `ghcr.io/bethropolis/localgo:scratch` | `scratch` | ~10 MB | No |

The **scratch** image is recommended for production. It contains only the Go binary and CA certificates — no shell, no package manager, minimal CVE surface. For development or when you need shell access, use the **standard** image.

---

## Quick Start

### Scratch image (recommended)

```bash
mkdir -p downloads config
docker run -d \
  --name localgo \
  --network host \
  -v $(pwd)/downloads:/app/downloads:z \
  -v $(pwd)/config:/app/config:z \
  -e PUID=$(id -u) -e PGID=$(id -g) \
  ghcr.io/bethropolis/localgo:scratch
```

### Standard image (with shell)

```bash
mkdir -p downloads config
docker run -d \
  --name localgo \
  --network host \
  -v $(pwd)/downloads:/app/downloads:z \
  -v $(pwd)/config:/app/config:z \
  ghcr.io/bethropolis/localgo:latest
```

### Docker Compose

```bash
docker compose up -d     # start
docker compose logs -f  # follow logs
docker compose down     # stop
```

Use `docker compose -f docker-compose.scratch.yml up -d` for the scratch image.

---

## Image Variants

### Scratch (`localgo:scratch`)

- **No shell** — no `/bin/sh`, no `su-exec`, no package manager
- **Go-native permission fix** — `localgo docker-start` reads `PUID`/`PGID`, `chown`s the volume mounts, then drops privileges via `syscall.Setuid`/`Setgid`
- **Smaller attack surface** — ideal for security-conscious deployments
- **Requires `PUID`/`PGID`** — must be set so the binary can fix permissions before dropping privileges

### Standard (`localgo:latest`)

- **Alpine-based** — includes `/bin/sh`, `su-exec`, `wget`
- **Shell access** — `docker exec -it localgo /bin/sh`
- **Entrypoint script** — handles Podman/Docker uid_map detection and permission fixes automatically
- No special environment variables required

---

## Permissions — How It Works

### Scratch image (`docker-start`)

The `localgo docker-start` command (used as the container ENTRYPOINT):

1. Reads `PUID` and `PGID` from environment (default `1000`)
2. `os.Chown` on `/app/downloads` and `/app/config` to match the UID/GID
3. `syscall.Setgid(pgid)` then `syscall.Setuid(puid)`
4. `exec`s `localgo serve` — the process replaces itself with the unprivileged UID

```bash
-e PUID=$(id -u) -e PGID=$(id -g)
```

### Standard image (entrypoint script)

The entrypoint detects the runtime and acts accordingly:

| Scenario | Detection | Action |
|---|---|---|
| Non-root user (`--user`) | `id -u != 0` | exec directly |
| Rootless Podman (default) | `uid_map` shows UID 0 → host UID N | `chown -R 0:0` volumes (re-maps to host user on-disk), exec as UID 0 |
| Rootful Docker / Podman | `uid_map` shows UID 0 → host UID 0 | `chown -R $PUID:$PGID` volumes, drop to `localgo` via `su-exec` |

For rootful Docker, set `PUID`/`PGID` to match your host user:

```bash
-e PUID=$(id -u) -e PGID=$(id -g)
```

---

## Volume Mounts

| Mount | Purpose |
|---|---|
| `/app/downloads` | Received files (must be writeable) |
| `/app/config` | TLS certificates + device fingerprint (persist across restarts) |

Always create the host directories before starting the container (Podman does not auto-create them):

```bash
mkdir -p downloads config
```

> **`:z` is required on SELinux hosts** (Fedora, RHEL, CentOS). It is harmless on non-SELinux systems.

---

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `LOCALSEND_ALIAS` | Device name shown to peers | hostname |
| `LOCALSEND_PORT` | Listening port | `53317` |
| `LOCALSEND_PIN` | Require a PIN from senders | (none) |
| `LOCALSEND_FORCE_HTTP` | Disable HTTPS | `false` |
| `LOCALSEND_AUTO_ACCEPT` | Accept files without prompting | `false` |
| `LOCALSEND_NO_CLIPBOARD` | Save text as file instead of clipboard | `false` |
| `LOCALSEND_DEVICE_TYPE` | `mobile` / `desktop` / `server` / `headless` / … | `desktop` |
| `LOCALSEND_DEVICE_MODEL` | Model string | `LocalGo` |
| `LOCALSEND_LOG_LEVEL` | `debug` / `info` / `warn` / `error` | `info` |
| `PUID` | Host user ID for file ownership (scratch + rootful Docker) | `1000` |
| `PGID` | Host group ID for file ownership (scratch + rootful Docker) | `1000` |
| `SKIP_PERMS_FIX` | Skip the entrypoint permission fix (standard image only) | `false` |

> **Clipboard note:** No clipboard tools are present in either image. Incoming `text/plain` transfers are saved as `.txt` files automatically.

---

## Network Configuration

### Linux — host network (recommended)

```bash
--network host
```

Required for multicast device discovery. The container shares the host's network interfaces directly.

### macOS / Windows

Docker and Podman run inside a VM on these platforms, so `--network host` binds to the VM's network, not the host computer's. Multicast discovery will not reach the real LAN.

**Option 1 — Port mapping** (simple but multicast still broken):

```bash
-p 53317:53317/tcp -p 53317:53317/udp
```

**Option 2 — Macvlan** (recommended for Mac/Windows) — gives the container a real LAN IP:

Macvlan gives the container its own IP address on your actual home router, bypassing the VM entirely. Create `docker-compose.macvlan.yml`:

```bash
NETWORK_INTERFACE=eth0 docker compose -f docker-compose.macvlan.yml up -d
```

Set `NETWORK_INTERFACE` to your host's network interface name (`eth0`, `en0`, `wlan0`, etc.). Assign a static IP on your router's DHCP reservation.

---

## Read-Only Root Filesystem

The scratch image supports a fully read-only root filesystem. Only `/app/downloads` and `/app/config` need to be writeable:

```yaml
read_only: true
security_opt:
  - no-new-privileges:true
volumes:
  - ./downloads:/app/downloads:z
  - ./config:/app/config:z
```

The Go binary writes logs to `stdout`/`stderr` (captured by `docker logs`), so no log directory is needed.

---

## Health Check

Both images expose a built-in health check via `localgo health`:

```bash
docker inspect localgo | jq '.[0].State.Health.Status'
podman inspect localgo --format '{{.State.Health.Status}}'
```

`localgo health` hits `http://127.0.0.1:{port}/api/localsend/v2/info` and exits `0` on HTTP 200, `1` otherwise. The scratch image works because the health check is compiled into the binary — no external tools required.

If you enable `LOCALSEND_FORCE_HTTP=false` to use TLS, the health check still works (it hits HTTP, the server always listens on HTTP for the API regardless of TLS settings for file transfers).

---

## Graceful Shutdown

LocalGo handles `SIGTERM` gracefully:

- The `serve` command listens for `SIGINT`/`SIGTERM`
- On signal, it calls `httpServer.Shutdown()` — rejecting new uploads
- Active file transfers complete or are cleanly aborted (`.part` files are removed)
- The transfer history (`history.jsonl`) is flushed to disk

When using `docker compose stop`, Docker sends `SIGTERM` and waits up to 10 seconds. LocalGo exits promptly, so active transfers are cleanly handled.

---

## Auto-Update (Watchtower / Diun)

Both images include OCI labels for automated updater tools:

```dockerfile
LABEL com.centurylinklabs.watchtower.enable="true"
LABEL org.opencontainers.image.ref.name="localgo"
LABEL org.opencontainers.image.vendor="Bethropolis"
```

Tools like [Watchtower](https://github.com/containrrr/watchtower) or [Diun](https://github.com/crazy-max/diun) will automatically detect and update LocalGo containers when a new image is pushed.

---

## Building Locally

### Standard image

```bash
docker build -t localgo .
```

### Scratch image

```bash
docker build -f Dockerfile.scratch -t localgo:scratch .
```

### With version metadata

```bash
docker build \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  -t ghcr.io/bethropolis/localgo:scratch \
  -f Dockerfile.scratch .
```

For Podman, replace `docker` with `podman`.

---

## Troubleshooting

### `lstat ...: no such file or directory`

Podman doesn't auto-create bind-mount source directories. Create them first:

```bash
mkdir -p downloads config
```

### Permission denied writing to `/app/downloads`

1. Make sure you created the directories **before** running the container.
2. On SELinux, add `:z` to volume flags.
3. For scratch or rootful Docker, set your UID/GID:
   ```bash
   -e PUID=$(id -u) -e PGID=$(id -g)
   ```

### Multicast discovery not working

- Linux: verify `--network host` is set.
- Check your firewall allows UDP port 53317.
- Use HTTP scan as fallback: `docker exec localgo localgo scan`

### Container generates a new fingerprint on every restart

If `/app/config` is not mounted to a persistent path, a new TLS certificate is generated on each start. Other LocalSend clients will see it as a different device. Always mount `/app/config`.

---

## Security Recommendations

- Set `LOCALSEND_PIN` in any environment with untrusted peers.
- Mount `/app/config` to persist a stable device fingerprint.
- Use the **scratch** image for production (minimal attack surface).
- Enable `read_only: true` where your orchestrator supports it.
- Set `LOCALSEND_AUTO_ACCEPT=false` if you want manual approval.

---

## See Also

- [Configuration Guide](CONFIGURATION.md)
- [Getting Started](GETTING_STARTED.md)
- [CLI Reference](CLI_REFERENCE.md)
- [Deployment Guide](DEPLOYMENT.md)
