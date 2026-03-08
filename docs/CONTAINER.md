# Docker & Podman Deployment Guide

LocalGo supports both Docker and rootless Podman out of the box.

---

## Quick Start

### Podman (recommended on Linux)

```bash
# Create the data directories first — Podman does not auto-create them
mkdir -p downloads config

# Run from the pre-built image
podman run -d \
  --name localgo \
  --network host \
  -v $(pwd)/downloads:/app/downloads:z \
  -v $(pwd)/config:/app/config:z \
  -e LOCALSEND_ALIAS="My Server" \
  ghcr.io/bethropolis/localgo:latest
```

> **`:z` is required on SELinux hosts** (Fedora, RHEL, CentOS).  
> It is harmless on non-SELinux systems, so always include it.

### Docker

```bash
mkdir -p downloads config

docker run -d \
  --name localgo \
  --network host \
  -v $(pwd)/downloads:/app/downloads \
  -v $(pwd)/config:/app/config \
  -e LOCALSEND_ALIAS="My Server" \
  ghcr.io/bethropolis/localgo:latest
```

### Docker Compose

```bash
docker-compose up -d    # start in background
docker-compose logs -f  # follow logs
docker-compose down     # stop and remove
```

---

## Volume Mounts

| Mount | Purpose |
|---|---|
| `/app/downloads` | Received files are written here |
| `/app/config` | TLS certificate + device fingerprint (persists across restarts) |

**Always create the host directories before starting the container.**  
Podman errors with `lstat ...: no such file or directory` if they are missing.

```bash
mkdir -p downloads config
```

### Permissions — how the entrypoint handles them

The entrypoint detects which runtime it is in and acts accordingly:

| Scenario | Detection | Action |
|---|---|---|
| Non-root user (`--user`) | `id -u != 0` | exec directly |
| Rootless Podman (default) | `uid_map` shows UID 0 → host UID N | `chown -R 0:0` volumes (remaps to host user on-disk), exec as UID 0 |
| Rootful Docker / Podman | `uid_map` shows UID 0 → host UID 0 | `chown -R $PUID:$PGID` volumes, drop to `localgo` via `su-exec` |

For rootless Podman no extra configuration is needed — the container process
runs as UID 0, which the kernel maps to your real host user.

For rootful Docker, set `PUID`/`PGID` to match your host user if you want
downloaded files owned by you:

```bash
docker run -d \
  --network host \
  -v $(pwd)/downloads:/app/downloads \
  -e PUID=$(id -u) \
  -e PGID=$(id -g) \
  ghcr.io/bethropolis/localgo:latest
```

---

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `LOCALSEND_ALIAS` | Device name shown to peers | Hostname |
| `LOCALSEND_PORT` | Listening port | `53317` |
| `LOCALSEND_PIN` | Require a PIN from senders | (none) |
| `LOCALSEND_FORCE_HTTP` | Disable HTTPS | `true` (in container) |
| `LOCALSEND_AUTO_ACCEPT` | Accept files without prompting | `true` (in container) |
| `LOCALSEND_NO_CLIPBOARD` | Save text as file instead of clipboard | `false` |
| `LOCALSEND_DEVICE_TYPE` | `mobile` / `desktop` / `server` / `headless` / … | `desktop` |
| `LOCALSEND_DEVICE_MODEL` | Model string | `LocalGo` |
| `LOCALSEND_LOG_LEVEL` | `debug` / `info` / `warn` / `error` | `info` |
| `LOCALSEND_LOG_FORMAT` | `json` (structured) or `text` | `json` (in container) |
| `PUID` | Host user ID for file ownership (rootful only) | `1000` |
| `PGID` | Host group ID for file ownership (rootful only) | `1000` |
| `SKIP_PERMS_FIX` | Skip the entrypoint chown step entirely | `false` |

> **Clipboard note:** No clipboard tools (`xclip`, `wl-copy`, etc.) are present
> in the image. Incoming `text/plain` transfers are automatically saved as
> `.txt` files in the download directory instead. No configuration needed.

---

## Building Locally

```bash
# Standard build
docker build -t localgo .

# With version metadata
docker build \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  -t localgo:$(git describe --tags --always) .
```

For Podman, replace `docker` with `podman` and use `Containerfile` if preferred:

```bash
podman build -f Containerfile -t localgo .
```

---

## Network Configuration

### Linux — host network (recommended)

```bash
--network host
```

Required for multicast device discovery. The container shares the host's
network interfaces directly.

### macOS / Windows

Podman and Docker both run inside a VM on these platforms, so `--network host`
does not give direct LAN access. Use port mapping instead:

```bash
-p 53317:53317/tcp \
-p 53317:53317/udp
```

Multicast discovery is unreliable across the VM boundary. Use HTTP scanning
as a fallback:

```bash
podman exec localgo localgo scan
```

---

## Health Check

The image exposes an HTTP healthcheck on `/api/localsend/v2/info`.
`LOCALSEND_FORCE_HTTP` defaults to `true` in the container image so the
healthcheck uses plain HTTP without any certificate handling.

```bash
# Inspect health status
podman inspect localgo --format '{{.State.Health.Status}}'
docker inspect localgo | jq '.[0].State.Health.Status'
```

If you set `LOCALSEND_FORCE_HTTP=false` to enable TLS inside the container,
update the healthcheck in `docker-compose.yml` to use `https://` with
`--no-check-certificate`.

---

## Troubleshooting

### `lstat ...: no such file or directory`

Podman doesn't auto-create bind-mount source directories. Create them first:

```bash
mkdir -p downloads config
```

### `permission denied` writing to `/app/downloads`

1. Make sure you created the directories **before** running the container.
2. On SELinux hosts (Fedora, RHEL, CentOS), add `:z` to your volume flags:
   ```bash
   -v $(pwd)/downloads:/app/downloads:z \
   -v $(pwd)/config:/app/config:z
   ```
3. If running rootful Docker and files appear owned by root on the host, pass
   your user ID:
   ```bash
   -e PUID=$(id -u) -e PGID=$(id -g)
   ```

### Multicast discovery not working

- Verify `--network host` is set (Linux only).
- Check your firewall allows UDP port 53317.
- Use HTTP scan as a fallback: `podman exec localgo localgo scan`

### Container generates a new fingerprint on every restart

If `/app/config` is not mounted to a persistent path, a new TLS certificate
and device fingerprint is generated on every start. Other LocalSend clients
will see it as a different device each time. Always mount `/app/config`.

---

## Security Recommendations

- Set `LOCALSEND_PIN` in any environment accessible to untrusted peers.
- Mount `/app/config` to persist a stable device fingerprint across restarts.
- Use `LOCALSEND_AUTO_ACCEPT=false` with manual approval if you want to
  control which transfers are accepted.

---

## See Also

- [Configuration Guide](CONFIGURATION.md) — all environment variables in detail
- [Getting Started](GETTING_STARTED.md) — initial setup
- [CLI Reference](CLI_REFERENCE.md) — command-line flags