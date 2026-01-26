# Deployment Guide

This guide covers various ways to deploy and run LocalGo, from simple binaries to containerized setups.

## 🐳 Docker Deployment

LocalGo provides official Docker images via GitHub Container Registry (GHCR).

### Basic Run
The simplest way to run LocalGo in a container:

```bash
docker run -d \
  --name localgo \
  --restart unless-stopped \
  -p 53317:53317 \
  -p 53317:53317/udp \
  -v ./downloads:/app/downloads \
  -v ./localgo_data:/app/.localgo_security \
  -e LOCALSEND_ALIAS="My-Docker-Node" \
  ghcr.io/bethropolis/localgo:latest
```

**Notes:**
- We expose `53317` on both TCP (file transfer) and UDP (discovery).
- We mount volumes for `downloads` (files you receive) and `.localgo_security` (to persist your identity/fingerprint).

---

### Docker Compose (Recommended)

For a persistent and reproducible setup, use Docker Compose.

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  localgo:
    image: ghcr.io/bethropolis/localgo:latest
    container_name: localgo
    restart: unless-stopped
    ports:
      # TCP for file transfer, UDP for multicast discovery
      - "53317:53317"
      - "53317:53317/udp"
    volumes:
      # Persist received files
      - ./downloads:/app/downloads
      # Persist SSL certs and config fingerprint
      - ./localgo_data:/app/.localgo_security
    environment:
      - LOCALSEND_ALIAS=Docker-Server
      - LOCALSEND_DEVICE_TYPE=server
      # Optional: PIN protection
      # - LOCALSEND_PIN=12345
```

**Start the service:**
```bash
docker-compose up -d
```

**View logs:**
```bash
docker-compose logs -f
```

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
    localgo-cli serve
    ```

**Tip:** Use `nohup` or `screen`/`tmux` to keep it running after you disconnect:
```bash
nohup localgo-cli serve > localgo.log 2>&1 &
```
