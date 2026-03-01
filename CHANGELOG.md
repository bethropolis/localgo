# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.2.0] - 2026-03-01

### Added
- **Multi-file Transfers**: Send multiple files concurrently with parallel uploads
- **Download API**: New `share` command for web-based file downloads (reverse transfer)
- **Interactive Prompts**: Accept/reject incoming file transfers via CLI
- **Peer Registry**: Device tracking with `devices` command
- **File Metadata**: Timestamps preserved on receive
- **Podman Support**: Containerfile and improved rootless container support
- **Docker Improvements**: HTTP healthcheck, STOPSIGNAL, better entrypoint
- **systemd**: Fixed IPAddressAllow bug, user service file now tracked

### Fixed
- LOCALSEND_FORCE_HTTP, DEVICE_TYPE, DEVICE_MODEL env vars now parsed
- Go version requirement updated to 1.24
- Various systemd service improvements

## [v0.1.3] - 2026-01-28

### Added
- Continuous integration and release automation
- Additional protocol compliance improvements

### Added
- **Docker Support:**
    - Non-root user execution with configurable UID/GID managed by a new entrypoint script.
    - Comprehensive `docker-compose.yml` with health checks.
    - New Docker deployment guide (`docs/DOCKER.md`).
    - GitHub Actions workflow for publishing to GHCR.
- **System Integration:**
    - XDG-compliant security directory resolution (config/certs now stored in standard locations).
    - User-mode systemd service (`localgo.service`) with hardening options.
    - Fish shell completion script (`scripts/fish_completion.fish`).
    - Improved install script robustness.
- **Documentation:**
    - New `CODE_WALKTHROUGH.md` guide.
    - Comprehensive `DEPLOYMENT.md` guide.
    - CLI help system refactored with ANSI colored output.

### Fixed
- Binary build location changed to current directory instead of `/tmp`.

## [v0.1.1] - 2026-01-25

### Added
- Dedicated GitHub Actions for testing and releases.
- Release artifacts are now flattened into a `dist` directory.
- Comprehensive documentation updates.

### Fixed
- **Discovery:** Enabled reliable bidirectional discovery.
- **Protocol:** Improved compliance with LocalSend v2.1 specification.

## [v0.1.0] - 2026-01-24

### Added
- Initial release.
- Go global install method in README.
- Basic CLI functionality for send, receive, discover, and scan.
