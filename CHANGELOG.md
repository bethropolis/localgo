# Changelog

All notable changes to this project are documented in this file.

## v0.6.0 - 2026-06-24

### Highlights
- **Protocol audit**: full spec compliance pass — `ProtocolVersion` 2.1→2.0, session blocking (409), `POST /register`, constant-time PIN, correct fingerprint selection, DTO field cleanup, `Port`/`Protocol` in InfoDto, and more
- **Modularisation**: 6 files exceeding 300 LOC split into 19 single-responsibility units for maintainability
- **FreeBSD support**: rc.d init script and clipboard integration (`clipboard_unix.go` with `linux||freebsd` build tag)
- **`--no-color` flag** and automatic `NO_COLOR` env var detection in logging
- **Direct send & CIDR scan**: `localgo send --ip <address>` and `localgo scan --range <CIDR>` flags
- **TUI file picker**: `localgo share` now opens an interactive file picker via `huh.FilePicker`
- **Gateway-based subnet prioritization**: smarter LAN discovery and scanning
- **GitHub Pages docs site** and online one-liner installer (`get-localgo.sh`)
- **Scratch Docker image hardened**: CMD args fixed, env vars set for writable peer cache

### Added
- `--no-color`/`--no-colour` global flag, `NO_COLOR` env support (`pkg/logging`)
- FreeBSD rc.d init script for `localgo serve` as a service
- FreeBSD clipboard support via `clipboard_unix.go` (`linux || freebsd`)
- `send --ip <address>` flag for direct IP-based send (skips discovery)
- `scan --range <CIDR>` flag for CIDR-based subnet scanning
- `ParseCIDRRange()` exported from `pkg/network/interfaces.go`
- `SendToDevice()` exported from `pkg/send/send.go` for programmatic use
- Gateway-based LAN subnet prioritization for scan and send
- Interactive TUI file picker in `share` command (extracted shared picker to `pkg/cli`)
- GitHub Pages docs site (`gh-pages` branch) and online installer
- `XDG_CACHE_HOME` env var for writable peer cache in scratch Docker
- `LOCALSEND_AUTO_ACCEPT=true` env var for scratch Docker image
- Homebrew cask support via goreleaser `homebrew_casks`

### Fixed (Protocol Audit)
- `ProtocolVersion` correctly set to `"2.0"` (was `"2.1"`) to match the LocalSend spec
- Session blocking: return 409 Conflict for concurrent sessions on same device
- Validate `?sessionId` in `PrepareDownloadHandler`
- Use `POST /register` instead of deprecated `GET /info` for HTTP subnet scan
- Constant-time PIN comparison in `DownloadHandler`
- Correct fingerprint selection in HTTP mode (random string, not certificate hash)
- Add `Port`/`Protocol` to prepare-upload `InfoDto` per spec section 4.1
- Use valid `deviceType "headless"` in private mode
- Remove spec-noncompliant extra fields from DTO structs
- Return no body on upload/cancel responses
- Force HTTP for `share` command (browser download API compatibility)
- Verify TLS certificate fingerprint during file transfer (MitM prevention)

### Fixed (Other)
- Case-insensitive TLS fingerprint comparison
- Remove duplicate `-p` shorthand in `devices` command
- Clipboard prompt removed from `send`; filepicker is the default TUI fallback
- HTTP subnet scan fallback when multicast returns 0 devices
- Filter local machine out of HTTP scan results
- Send multicast response via multicast address instead of unicast
- Check `xdg-open` availability before opening download directory
- Scratch Docker: CMD args pass-through (no double `"localgo"`), `LOCALSEND_DOWNLOAD_DIR` and `LOCALSEND_SECURITY_DIR` env vars
- `DiscoverDevices` private mode bypass in `cmd/send.go`
- Device mutex for `LastSeen`/`Available`, `ReceiveService` ticker goroutine leak
- Config set parsing, scan/discover timeouts, share port order, CIDR range, RNG fallback
- PIN constant-time compare, server timeouts, private mode DTO bypass, JPEG bounds strip
- Progress bar scrollback erasure fix, bounds-safe `FormatBytes` (no panic on >EB sizes)
- Storage: atomic file writes via `.tmp` rename pattern; Windows: lazy DLL loading (`NewLazyDLL`)

### Refactored
- 6 files exceeding 300 LOC split into 19 smaller single-responsibility units
- Shared TUI file picker extracted to `pkg/cli`
- Code quality: `SortFunc`, mutex-safe anonymize, `saveTextAsFile`, interface extraction, tests

### Commits (v0.5.10..v0.6.0)
- `814b5fd` refactor: split 6 large files into 19 single-responsibility units
- `0348ddb` chore: stable release prep — bugs, atomic writes, safety
- `b43e423` feat: add GitHub Pages docs site and online installer
- `16da01b` fix: stability fixes and enhancements
- `51de7a2` fix: remove duplicate -p shorthand in devices command
- `53ffe3d` feat(share): add TUI file picker, extract shared picker to pkg/cli
- `3d9c9bb` fix: bug fix
- `c0edea8` fix: case-insensitive TLS fingerprint comparison
- `0f2c8ce` chore: final state after protocol audit fixes
- `68d35a9` fix: improve TLS error diag, always prompt device picker, silence usage on errors
- `d1af3c1` fix(protocol): force HTTP for share command (browser download API)
- `221bfda` fix(security): verify TLS certificate fingerprint during file transfer
- `52f39a8` fix(protocol): add port/protocol to prepare-upload info block
- `4825c46` refactor(dto): remove spec-noncompliant extra fields from DTO structs
- `0c4ea80` fix(protocol): use valid deviceType 'headless' in private mode, return no body on upload/cancel
- `fd65357` fix(protocol): validate ?sessionId in PrepareDownloadHandler
- `261b904` fix(protocol): implement session blocking, return 409 for concurrent sessions
- `beb3629` fix(discovery): use POST /register instead of deprecated GET /info for HTTP subnet scan
- `a08245a` fix(security): use constant-time PIN comparison in DownloadHandler
- `01be941` fix(protocol): select correct fingerprint in HTTP mode (random string, not cert hash)
- `c5b3a8d` fix(protocol): change ProtocolVersion from '2.1' to '2.0' to match spec
- `2f47675` fix(send): remove interactive clipboard prompt, filepicker is the default TUI fallback
- `cf37d46` fix(discover): fall back to HTTP subnet scan when multicast returns nothing
- `de481d0` fix(scan): filter local machine out of HTTP scan results
- `8d35b6c` fix(discovery): send multicast response via multicast addr instead of unicast back
- `32a628d` feat(network): add gateway-based LAN subnet prioritization for scan and send
- `47f61e2` fix: check xdg-open availability before opening download directory
- `3599891` feat(freebsd): add rc.d init script for localgo service
- `5f13a84` feat(freebsd): enable clipboard support via clipboard_unix.go (linux||freebsd)
- `7aaf291` feat(cli): add --no-color flag, respect NO_COLOR env in logging Init
- `97a0c4a` docs(help): add completion cmd, missing flags for serve/share/send, --private/--config options
- `138952b` fix(help): correct discover --timeout default from 5 to 10
- `8bfafe2` fix(security): bypass DiscoverDevices private mode in cmd/send.go
- `413bcd1` refactor(code quality): SortFunc, mutex-safe anonymize, saveTextAsFile, interfaces, tests
- `ad832f9` fix(concurrency): Device mutex for LastSeen/Available, ReceiveService ticker goroutine leak
- `64be12d` fix(logic): config set parsing, scan/discover timeouts, share port order, CIDR range, RNG fallback
- `9144f42` fix(security): PIN constant-time compare, server timeouts, private mode DTO bypass, strip JPEG bounds
- `2a8a00b` fix(scratch): add XDG_CACHE_HOME so peer cache is writable
- `f6ed6a5` fix(scratch): add LOCALSEND_AUTO_ACCEPT=true env var
- `b013c88` fix: create discovery DTOs after server binds port
- `37be6e8` fix(scratch): set LOCALSEND_DOWNLOAD_DIR and LOCALSEND_SECURITY_DIR env vars
- `c01ef58` fix: docker-start passes CMD args correctly (no double localgo)
- `be29c69` feat: add send --ip, scan --range flags, ParseCIDRRange, export SendToDevice
- `6f8a9cc` feat: add private mode, progress bar fixes, metadata stripping, and core improvements

## v0.4.0 - 2026-05-11

### Highlights
- **Nerd Font icons**: replaced emoji (✅ ❌ ⏳ ⚠️ ℹ️) with Nerd Font glyphs for a consistent monospace terminal look (`pkg/cli/icons.go`)
- **Systemd service fix**: removed `ConfigurationDirectory` (caused systemd to own `~/.config/localgo` as root), added explicit XDG env vars, fixed `EnvironmentFile` path; service now starts correctly under `systemd --user`
- **Fixed env template**: `localgo.env.example` changed from hardcoded `/home/user` to `$HOME/Downloads/localgo`
- **Go 1.24 → 1.26**: updated Dockerfiles, go.mod, README badge, install script minimum version check, and all CI workflows
- **Reproducible container builds**: all Dockerfiles now use `-mod=vendor` with vendored source, bypassing module proxy entirely
- **Removed sqweek/dialog**: native file picker removed; use `--file` flag for sending (CGO-free, smaller binaries, simpler CI)

### Fixed
- Container health check now uses HTTPS with `--no-check-certificate` (HTTP returns 400 Bad Request)
- `localgo health` exit code (was 400, now 0)
- `podman-compose up` healthcheck syntax fixed (`CMD-SHELL` required in compose format)

### Added
- `localgo info` uses new Nerd Font icon styles
- All CLI output functions (`PrintSuccess`, `PrintError`, `PrintWarning`, `PrintInfo`, `WriteProgress`, `WriteSuccess`, `WriteWarning`) now use Nerd Font icons

### Refactored
- `PickFiles()` removed; `localgo send` requires `--file` flag explicitly

## v0.3.6 - 2026-05-04

### Refactored
- Extracted DTO factory methods to `pkg/config/dto.go`
- Moved `resolveDuplicateFilename` to `pkg/storage`
- Added progress bar helper in `pkg/cli/progress.go`
- Reduced boilerplate across major CLI commands

## v0.3.5 - 2026-03-04

### Highlights
- Binary renamed from `localgo-cli` to `localgo` — cleaner, simpler invocation
- Clipboard integration: incoming `text/plain` transfers are now copied to the system clipboard automatically
- Android arm build targets added to the release pipeline
- Systemd service hardening with resource limits
- Help system and all documentation fully audited and updated to match the actual CLI

### Added
- **Clipboard support**: incoming `text/plain` file transfers are now automatically copied to the system clipboard when a display server is available. Falls back to saving as a `.txt` file on headless systems (`pkg/clipboard`)
- **`--no-clipboard` flag** on `serve` and `share`: opt out of clipboard behaviour and always save text transfers to disk instead
- **`LOCALSEND_NO_CLIPBOARD` env var**: persistent alternative to `--no-clipboard`
- **Android armv7 and armv8 build targets** in the Makefile release pipeline (`GOOS=linux GOARCH=arm GOARM=7` / `GOARM=8`)
- `localgo help share` and `localgo help devices` now work (both commands were silently missing from `GetCommandHelp`)
- Global `--verbose` and `--json` flags now documented in `localgo help` output
- Full env var list (`LOCALSEND_NO_CLIPBOARD`, `LOCALSEND_DEVICE_MODEL`, `LOCALSEND_AUTO_ACCEPT`, `LOCALSEND_LOG_LEVEL`, etc.) shown in `localgo help`

### Changed
- **Binary renamed**: `localgo-cli` → `localgo` across the entire codebase — directory (`cmd/localgo`), Makefile, install script, systemd units, completions, Docker, CI, and all documentation
- **`go install` path** updated to `github.com/bethropolis/localgo/cmd/localgo@latest`
- `help.go` `ShowMainUsage()` COMMANDS list now includes `share` and `devices`
- `serve` help entry now documents `--interval`, `--auto-accept`, and `--no-clipboard`
- `send --file` description corrected: "File or directory to send (can be specified multiple times)"
- Release Makefile target refactored from complex `$(eval …)` macros to a clean shell `for` loop
- CI release workflow simplified to a single job using `make release`
- Systemd units tightened: `MemoryMax=128M`, `TasksMax=64`, `CPUSchedulingPolicy=idle`, `IOSchedulingClass=idle`, `Nice=15`, `LimitNOFILE=4096`, `StandardOutput=null`

### Fixed
- Fixed `localgo help share` and `localgo help devices` incorrectly printing "Unknown command"

### Documentation
- **CLI Reference** (`docs/CLI_REFERENCE.md`): Fully rewritten. Added complete flag tables for all commands (`serve`, `send`, `discover`, `scan`) and removed phantom flags that didn't exist in the code. Added a Global Flags section.
- **Configuration** (`docs/CONFIGURATION.md`): Fully updated. Ensured all command flags (like `send --port` and `share --no-clipboard`) are documented. Corrected the default `LOCALSEND_DEVICE_TYPE` to `"desktop"`. 
- **Getting Started** (`docs/GETTING_STARTED.md`): Expanded guides to cover `share`, `devices`, and `info` commands. Added guidance for headless setups, `--no-clipboard` usage, auto-accept scenarios, and a JSON scripting example.
- **Readme** (`README.md`): Added Clipboard Integration to the features list and documented new environment variables (`LOCALSEND_DEVICE_MODEL`, `LOCALSEND_AUTO_ACCEPT`, `LOCALSEND_NO_CLIPBOARD`, `LOCALSEND_LOG_LEVEL`).

### Commits (v0.3.2..v0.3.5)
- `6b7a69f` feat: add clipboard copy support for incoming text transfers
- `2facd93` chore(release): refactor release target and fix android armv7 build
- `61b041c` feat(release): add android armv7 and armv8 build targets
- `4358f23` chore: optimize service resources, simplify CI, and tune quiet logging
- `1ca64b6` fix(scripts): user service by default, fix BUILD_TMP scope, add --mode to uninstall, improve completion and verification

---

## v0.3.2 - 2026-03-02

### Highlights
- Fix multicast test timeout on CI by skipping when multicast delivery is unavailable
- Improve developer experience: coloured console logging, Air dev workflow, and a reworked Makefile
- CI hardening: remove flaky Trivy step and pin/update GitHub Actions; bump Go to 1.24
- Lots of tests added and several bug fixes across discovery, server, and send codepaths

### Added
- Add many unit tests for CLI, crypto, httputil, server handlers, model mapping, network helpers

### Changed / Improved
- Coloured human-readable console logging by default and `--json` flag for JSON output
- Replace logrus with zap for logging internals
- Rework Makefile with helpful targets: build, release, test variants, dev, and more

### Fixed
- Skip multicast tests in CI/sandbox when sockets bind but multicast delivery fails; skip on send failure
- Fix various server issues: data races, path traversal, pin validation, session leaks, and wait for HTTP bind before announcing discovery
- Fix send/handler issues: proper config passing, upload contexts, and duplicate filename handling
- Fix model defaults and file metadata mapping

### CI
- Remove flaky Trivy scanning step from Docker workflow; pin and upgrade Trivy action versions where appropriate
- Upgrade `docker/build-push-action` to v6 and adjust `codeql-action` version
- Bump GitHub Actions Go runner to 1.24

### Commits (v0.2.0..v0.3.2)
- `9a33cfb` fix(discovery): skip instead of fail on multicast delivery timeout in CI
- `d181cf2` ci(docker): remove Trivy scan
- `47c9d2a` ci(trivy): switch to stable trivy-action@0.33.1 due to recent installer issues
- `b2bbf84` ci(docker): restore Trivy scanning step
- `1c18650` ci(docker): remove Trivy scanning step (flaky external binary install)
- `806f207` ci(trivy): use tag format without leading 'v' (0.34.1) for aquasecurity/trivy-action
- `32a276d` fix(ci): bump trivy-action to v0.34.1
- `8d0ce24` fix(test): skip on send failure in TestMulticastDiscovery_ReceiveAnnouncement
- `7cb1572` fix(ci): pin trivy-action to 0.30.0, fix codeql action version, upgrade build-push to v6
- `9dcdab4` fix(ci): skip multicast tests when socket unavailable, bump Go to 1.24 in CI
- `ca319b8` feat(dx): coloured console logging, improved Makefile, add air + golangci-lint config
- `ea107c7` refactor(logging): replace logrus with zap
- `bb3c69e` test: add unit tests for pkg/cli, pkg/crypto, and pkg/httputil
- `950bbcc` fix(handlers): unify logging with logrus and add duplicate filename handling
- `f2f3487` fix(server): replace sleep-based ready signal with net.Listen port binding
- `ee0b69a` test(model): add tests for file DTO mapping and file type detection
- `11a313b` fix(model): correct default port to 53317, update file metadata mapping, and support MaxBodySize configuration
- `4359dde` test(network): add tests for local IP parsing and subnet calculation
- `17ce65f` test(discovery): add tests for multicast UDP discovery
- `0bcae8a` fix(discovery): fix multicast data races and use proper protocol scheme for http registration
- `09943f1` test(send): add unit tests for sending files including errors
- `4a317f4` fix(send): properly pass configuration and handle upload contexts and errors
- `2a4e4eb` test(server): add comprehensive tests for server handlers
- `ce1d4f1` fix(server): fix data races, path traversal, pin validation, and session leaks
- `fe906c1` fix(server): wait for HTTP server to bind before announcing discovery
- `9bd9125` Add tests, docs and serve auto-accept flag
