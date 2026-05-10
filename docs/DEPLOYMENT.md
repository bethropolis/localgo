# Deployment Guide

This guide covers various ways to deploy and run LocalGo, from simple binaries to containerized setups.

## Docker Deployment

LocalGo provides official images via GitHub Container Registry (GHCR).

### Image Variants

| Image | Tag | Base | Size | Notes |
|-------|-----|------|------|-------|
| Standard | `latest` | Alpine | ~25 MB | Includes shell, useful for debugging |
| Scratch | `scratch` | `scratch` | ~10 MB | No shell, production-recommended |

The **scratch** image is built from `Dockerfile.scratch`. It uses `localgo docker-start` to handle permission setup natively via `syscall.Setuid`/`Setgid`.

### Quick Run (Scratch)

```bash
mkdir -p downloads config
docker run -d \
  --name localgo \
  --restart unless-stopped \
  --network host \
  -v $(pwd)/downloads:/app/downloads:z \
  -v $(pwd)/config:/app/config:z \
  -e PUID=$(id -u) -e PGID=$(id -g) \
  ghcr.io/bethropolis/localgo:scratch
```

### Docker Compose (Recommended)

LocalGo ships with three compose files:

| File | Use case |
|------|---------|
| `docker-compose.yml` | Standard image on Linux |
| `docker-compose.scratch.yml` | Scratch image on Linux |
| `docker-compose.macvlan.yml` | Scratch image with macvlan networking (Mac/Windows) |

```bash
# Linux — standard image
docker compose up -d

# Linux — scratch image (production)
docker compose -f docker-compose.scratch.yml up -d

# macOS/Windows — macvlan (bypasses VM isolation)
NETWORK_INTERFACE=eth0 docker compose -f docker-compose.macvlan.yml up -d
```

### Building Locally

```bash
# Standard image
docker build -t localgo .

# Scratch image
docker build -f Dockerfile.scratch -t localgo:scratch .
```

### Health Check

Both images ship with a built-in health check. The scratch image uses `localgo health` directly — no shell or external tools needed:

```bash
docker inspect localgo | jq '.[0].State.Health.Status'
```

`localgo health` hits `http://127.0.0.1:53317/api/localsend/v2/info` and exits `0` on 200, `1` otherwise.

### Read-Only Root Filesystem (Scratch)

The scratch image supports `read_only: true` for maximum container security. Only `/app/downloads` and `/app/config` are writeable:

```yaml
read_only: true
security_opt:
  - no-new-privileges:true
volumes:
  - ./downloads:/app/downloads:z
  - ./config:/app/config:z
```

See [Container Documentation](CONTAINER.md) for full details on image variants, macvlan setup, and deployment.

---

## 🖥️ Systemd Service (Linux)

You can run LocalGo as a background service managed by `systemd`. We offer an installer script to set this up automatically.

### User Service (Recommended)
Runs as your current user. Best for desktops/laptops.

**Install:**
```bash
./scripts/install.sh --mode user --service
```

**Manage:**
```bash
systemctl --user start localgo
systemctl --user status localgo
journalctl --user -u localgo -f
```

### System Service
Runs as a dedicated `localgo` user. Best for headless servers or multi-user systems.

**Install:**
```bash
sudo ./scripts/install.sh --mode system --service --create-user
```

**Manage:**
```bash
sudo systemctl start localgo
sudo systemctl status localgo
```

---

## 📂 Manual Binary Deployment

If you prefer to manage the binary yourself:

1.  **Download** the latest release corresponding to your OS/Arch.
2.  **Move** it to a directory in your `$PATH` (e.g., `/usr/local/bin`).
3.  **Run** it directly:
    ```bash
    localgo serve
    ```

**Tip:** Use `nohup` or `screen`/`tmux` to keep it running after you disconnect:
```bash
nohup localgo serve > localgo.log 2>&1 &
```
