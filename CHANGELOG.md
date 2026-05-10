# Changelog

All notable changes to this project are documented in this file.

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
