#!/bin/bash
#
# LocalGo Online Installer
# Downloads and installs the latest pre-built LocalGo binary from GitHub Releases.
# No Go toolchain required. Works on Linux (amd64/arm64) and macOS (amd64/arm64).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/bethropolis/localgo/main/scripts/online-install.sh | bash
#   curl -fsSL ... | bash -s -- --mode system
#   curl -fsSL ... | bash -s -- --mode system --service --completion
#   curl -fsSL ... | bash -s -- --version 0.5.5
#

set -euo pipefail

# ── Constants ──────────────────────────────────────────────────────
BINARY_NAME="localgo"
GH_OWNER="bethropolis"
GH_REPO="localgo"
GH_URL="https://github.com/$GH_OWNER/$GH_REPO"
GH_API="https://api.github.com/repos/$GH_OWNER/$GH_REPO"

USER_BIN_DIR="$HOME/.local/bin"
USER_CONFIG_DIR="$HOME/.config/localgo"
USER_SERVICE_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"

SYSTEM_BIN_DIR="/usr/local/bin"
SYSTEM_CONFIG_DIR="/etc/localgo"
SYSTEM_SERVICE_DIR="/etc/systemd/system"

# ── Flags (defaults — all extras opt-in) ───────────────────────────
INSTALL_MODE="user"
INSTALL_SERVICE=false
INSTALL_COMPLETION=false
INSTALL_CONFIG=false
ASSUME_YES=false
DRY_RUN=false
PINNED_VERSION=""

# ── Runtime ────────────────────────────────────────────────────────
OS=""
ARCH=""
VERSION=""
TAG=""
TMPDIR=""
BINARY_PATH=""

# ── Colors ─────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m'

# ── Output helpers ─────────────────────────────────────────────────
info()  { echo -e "  ${BLUE}ℹ${NC}  $*"; }
ok()    { echo -e "  ${GREEN}✔${NC}  $*"; }
warn()  { echo -e "  ${YELLOW}⚠${NC}  $*"; }
err()   { echo -e "  ${RED}✖${NC}  $*" >&2; }
header(){ echo -e "\n  ${MAGENTA}◆${NC}  $*"; }

# ── Usage ─────────────────────────────────────────────────────────
usage() {
    cat <<EOF
LocalGo Online Installer — Download and install LocalGo from GitHub Releases.

USAGE:
    $0 [OPTIONS]

OPTIONS:
    -v, --version VER    Install a specific version (default: latest)
    --mode MODE          Install mode: user (default) | system
    --service            Install systemd user service (Linux only)
    --completion         Install shell completions (bash/zsh/fish)
    --config             Install example configuration file
    -y, --yes            Non-interactive; skip confirmation
    --dry-run            Print plan and exit
    -h, --help           Show this help message

MODES:
    user                 Install to ~/.local/bin (default, no sudo)
    system               Install to /usr/local/bin (requires sudo)

EXAMPLES:
    curl -fsSL $GH_URL/main/scripts/online-install.sh | bash
    curl -fsSL ... | bash -s -- --mode system --service --completion
    curl -fsSL ... | bash -s -- --version 0.5.5
EOF
    exit 0
}

# ── Helpers ────────────────────────────────────────────────────────
cleanup() {
    [[ -n "$TMPDIR" && -d "$TMPDIR" ]] && rm -rf "$TMPDIR"
}
trap cleanup EXIT INT TERM

die() {
    err "$*"
    exit 1
}

sudo_cmd() {
    if [[ "$INSTALL_MODE" == "system" ]]; then
        sudo "$@"
    else
        "$@"
    fi
}

confirm_or_skip() {
    [[ "$ASSUME_YES" == "true" ]] && return 0
    echo -n "  Proceed with installation? [y/N]: "
    local response=""
    [[ -t 0 ]] && read -r response
    case "$response" in
        [Yy]*) return 0 ;;
        *) warn "Installation canceled."; exit 0 ;;
    esac
}

# ── Step 1: Platform Detection ─────────────────────────────────────
detect_platform() {
    local os_raw arch_raw
    os_raw=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch_raw=$(uname -m)

    case "$os_raw" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        *)      die "Unsupported OS: $os_raw (expected linux or darwin)" ;;
    esac

    case "$arch_raw" in
        x86_64|amd64)    ARCH="amd64" ;;
        arm64|aarch64)   ARCH="arm64" ;;
        *) die "Unsupported architecture: $arch_raw (expected x86_64, amd64, arm64, or aarch64)" ;;
    esac

    ok "Platform detected: ${OS}_${ARCH}"
}

# ── Step 2: Version Resolution ─────────────────────────────────────
resolve_version() {
    if [[ -n "$PINNED_VERSION" ]]; then
        TAG="v${PINNED_VERSION#v}"
        VERSION="${TAG#v}"
        info "Using pinned version: $VERSION"
        return
    fi

    info "Resolving latest release from GitHub..."

    # Follow the /releases/latest redirect to avoid GitHub API rate limits
    local latest_url
    latest_url=$(curl -sIL -o /dev/null -w "%{url_effective}" \
        "https://github.com/$GH_OWNER/$GH_REPO/releases/latest" 2>/dev/null || true)

    if [[ -z "$latest_url" || "$latest_url" == *"/releases/latest" ]]; then
        # Fallback: GitHub JSON API
        info "Redirect resolution failed, falling back to GitHub API..."
        latest_url=$(curl -sL "$GH_API/releases/latest" 2>/dev/null \
            | grep '"tag_name":' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/' || true)
        [[ -z "$latest_url" ]] && die "Failed to resolve latest version. Use --version to specify one."
        TAG="$latest_url"
    else
        TAG=$(echo "$latest_url" | sed 's|.*/||')
    fi

    VERSION="${TAG#v}"
    ok "Latest release: $TAG"
}

# ── Step 3: Print Plan ─────────────────────────────────────────────
print_plan() {
    local dest
    if [[ "$INSTALL_MODE" == "system" ]]; then
        dest="$SYSTEM_BIN_DIR/$BINARY_NAME"
    else
        dest="$USER_BIN_DIR/$BINARY_NAME"
    fi

    echo
    echo "  ┌─ Installation Plan ──────────────────────────────────┐"
    printf "  │  %-20s %-30s  │\n" "Version:"     "$TAG"
    printf "  │  %-20s %-30s  │\n" "Platform:"    "${OS}_${ARCH}"
    printf "  │  %-20s %-30s  │\n" "Destination:" "$dest"
    printf "  │  %-20s %-30s  │\n" "Service:"     "$( [[ $INSTALL_SERVICE == true ]] && echo yes || echo no )"
    printf "  │  %-20s %-30s  │\n" "Completions:" "$( [[ $INSTALL_COMPLETION == true ]] && echo yes || echo no )"
    printf "  │  %-20s %-30s  │\n" "Config:"      "$( [[ $INSTALL_CONFIG == true ]] && echo yes || echo no )"
    echo "  └──────────────────────────────────────────────────────┘"
    echo

    if [[ "$INSTALL_MODE" == "system" && $EUID -ne 0 ]]; then
        info "System mode: sudo will be used for file operations."
    fi
}

# ── Step 4: Download + Verify ──────────────────────────────────────
download_and_verify() {
    header "Downloading LocalGo $TAG..."

    TMPDIR=$(mktemp -d)
    local archive_name="localgo_${VERSION}_${OS}_${ARCH}.tar.gz"
    local archive_url="$GH_URL/releases/download/$TAG/$archive_name"
    local checksum_url="$GH_URL/releases/download/$TAG/checksums.txt"

    info "Downloading archive: $archive_name"

    if command -v curl &>/dev/null; then
        curl -fsSL "$archive_url" -o "$TMPDIR/$archive_name" || die "Download failed: $archive_url"
        curl -fsSL "$checksum_url" -o "$TMPDIR/checksums.txt" 2>/dev/null || warn "Checksums file not found, skipping verification"
    elif command -v wget &>/dev/null; then
        wget -qO "$TMPDIR/$archive_name" "$archive_url" || die "Download failed: $archive_url"
        wget -qO "$TMPDIR/checksums.txt" "$checksum_url" 2>/dev/null || warn "Checksums file not found, skipping verification"
    else
        die "Neither curl nor wget found. Install one of them and retry."
    fi

    ok "Archive downloaded"

    # ── Verify Checksum ──
    if [[ -f "$TMPDIR/checksums.txt" ]]; then
        local expected_hash
        expected_hash=$(grep "$archive_name" "$TMPDIR/checksums.txt" | awk '{print $1}')
        if [[ -n "$expected_hash" ]]; then
            local actual_hash=""
            if command -v sha256sum &>/dev/null; then
                actual_hash=$(sha256sum "$TMPDIR/$archive_name" | awk '{print $1}')
            elif command -v shasum &>/dev/null; then
                actual_hash=$(shasum -a 256 "$TMPDIR/$archive_name" | awk '{print $1}')
            fi
            if [[ -n "$actual_hash" ]]; then
                if [[ "$expected_hash" == "$actual_hash" ]]; then
                    ok "Checksum verified (SHA-256)"
                else
                    die "Checksum mismatch! Expected: $expected_hash, Got: $actual_hash"
                fi
            fi
        else
            warn "No checksum entry for $archive_name, skipping verification"
        fi
    fi

    # ── Extract ──
    info "Extracting archive..."
    tar -xzf "$TMPDIR/$archive_name" -C "$TMPDIR" || die "Failed to extract archive"
    ok "Archive extracted"

    # ── Locate Binary (handle both wrapped and flat archives) ──
    BINARY_PATH=$(find "$TMPDIR" -maxdepth 3 -type f -name "$BINARY_NAME" 2>/dev/null | head -1)
    [[ -z "$BINARY_PATH" ]] && die "Binary not found in archive"
    ok "Binary located: $(basename "$BINARY_PATH") v$VERSION"
}

# ── Step 5: Install Binary ─────────────────────────────────────────
install_binary() {
    header "Installing binary..."

    local dest_dir
    [[ "$INSTALL_MODE" == "system" ]] && dest_dir="$SYSTEM_BIN_DIR" || dest_dir="$USER_BIN_DIR"

    sudo_cmd mkdir -p "$dest_dir"
    sudo_cmd cp "$BINARY_PATH" "$dest_dir/$BINARY_NAME"
    sudo_cmd chmod 755 "$dest_dir/$BINARY_NAME"

    ok "Binary installed to $dest_dir/$BINARY_NAME"
}

# ── Helper: locate asset dir (scripts/ in archive) ─────────────────
find_asset_dir() {
    local dir
    dir=$(dirname "$BINARY_PATH")
    [[ -d "$dir/scripts" ]] && echo "$dir/scripts" && return
    dir=$(dirname "$dir")
    [[ -d "$dir/scripts" ]] && echo "$dir/scripts" && return
    echo ""
}

# ── Opt-in: Completions ────────────────────────────────────────────
install_completions() {
    header "Installing shell completions..."

    local scripts_dir
    scripts_dir=$(find_asset_dir)
    [[ -z "$scripts_dir" ]] && { warn "Completions not found in archive, skipping"; return; }

    local count=0

    if [[ -f "$scripts_dir/bash_completion.sh" ]] && command -v bash &>/dev/null; then
        if [[ "$INSTALL_MODE" == "system" ]]; then
            sudo mkdir -p /usr/share/bash-completion/completions
            sudo cp "$scripts_dir/bash_completion.sh" "/usr/share/bash-completion/completions/$BINARY_NAME"
        else
            mkdir -p "$HOME/.local/share/bash-completion/completions"
            cp "$scripts_dir/bash_completion.sh" "$HOME/.local/share/bash-completion/completions/$BINARY_NAME"
        fi
        ok "Bash completions installed"
        ((count++))
    fi

    if [[ -f "$scripts_dir/zsh_completion.zsh" ]] && command -v zsh &>/dev/null; then
        if [[ "$INSTALL_MODE" == "system" ]]; then
            sudo mkdir -p /usr/share/zsh/site-functions
            sudo cp "$scripts_dir/zsh_completion.zsh" "/usr/share/zsh/site-functions/_$BINARY_NAME"
        else
            mkdir -p "$HOME/.local/share/zsh/site-functions"
            cp "$scripts_dir/zsh_completion.zsh" "$HOME/.local/share/zsh/site-functions/_$BINARY_NAME"
        fi
        ok "Zsh completions installed"
        ((count++))
    fi

    if [[ -f "$scripts_dir/fish_completion.fish" ]] && command -v fish &>/dev/null; then
        if [[ "$INSTALL_MODE" == "system" ]]; then
            sudo mkdir -p /usr/share/fish/vendor_completions.d
            sudo cp "$scripts_dir/fish_completion.fish" "/usr/share/fish/vendor_completions.d/$BINARY_NAME.fish"
        else
            mkdir -p "$HOME/.config/fish/completions"
            cp "$scripts_dir/fish_completion.fish" "$HOME/.config/fish/completions/$BINARY_NAME.fish"
        fi
        ok "Fish completions installed"
        ((count++))
    fi

    [[ $count -eq 0 ]] && warn "No compatible shell found for completions"
}

# ── Opt-in: Service (Linux only, systemd required) ─────────────────
install_service() {
    header "Installing systemd service..."

    [[ "$OS" != "linux" ]] && { warn "systemd not available on macOS, skipping"; return; }
    command -v systemctl &>/dev/null || { warn "systemctl not found, skipping"; return; }

    local scripts_dir
    scripts_dir=$(find_asset_dir)
    [[ -z "$scripts_dir" ]] && { warn "Service file not found in archive, skipping"; return; }

    local service_src=""
    [[ -f "$scripts_dir/localgo-pkg.service" ]] && service_src="$scripts_dir/localgo-pkg.service"
    [[ -z "$service_src" && -f "$scripts_dir/localgo.service" ]] && service_src="$scripts_dir/localgo.service"
    [[ -z "$service_src" ]] && { warn "Service file not found in archive, skipping"; return; }

    local bin_path
    [[ "$INSTALL_MODE" == "system" ]] && bin_path="$SYSTEM_BIN_DIR/$BINARY_NAME" || bin_path="$USER_BIN_DIR/$BINARY_NAME"

    if [[ "$INSTALL_MODE" == "system" ]]; then
        local svc_dest="$SYSTEM_SERVICE_DIR/$BINARY_NAME.service"
        sudo mkdir -p "$SYSTEM_SERVICE_DIR"
        sudo cp "$service_src" "$svc_dest"
        sudo sed -i "s|ExecStart=.*|ExecStart=$bin_path serve --quiet --auto-accept|g" "$svc_dest" 2>/dev/null || true
        sudo sed -i "/^EnvironmentFile=/d" "$svc_dest" 2>/dev/null || true
        sudo systemctl daemon-reload 2>/dev/null || true
        ok "System service installed: $svc_dest"
    else
        local svc_dest="$USER_SERVICE_DIR/$BINARY_NAME.service"
        mkdir -p "$USER_SERVICE_DIR"
        cp "$service_src" "$svc_dest"
        sed -i "s|ExecStart=.*|ExecStart=$bin_path serve --quiet --auto-accept|g" "$svc_dest" 2>/dev/null || true
        sed -i "/^EnvironmentFile=/d" "$svc_dest" 2>/dev/null || true
        sed -i "/^User=/d" "$svc_dest" 2>/dev/null || true
        sed -i "/^Group=/d" "$svc_dest" 2>/dev/null || true
        systemctl --user daemon-reload 2>/dev/null || true
        ok "User service installed: $svc_dest"
    fi
}

# ── Opt-in: Config ─────────────────────────────────────────────────
install_config() {
    header "Installing configuration..."

    local scripts_dir
    scripts_dir=$(find_asset_dir)
    [[ -z "$scripts_dir" ]] && { warn "Config template not found in archive, skipping"; return; }

    local env_src="$scripts_dir/localgo.env.example"
    [[ ! -f "$env_src" ]] && { warn "Config template not found in archive, skipping"; return; }

    if [[ "$INSTALL_MODE" == "system" ]]; then
        sudo mkdir -p "$SYSTEM_CONFIG_DIR"
        if [[ ! -f "$SYSTEM_CONFIG_DIR/localgo.env" ]]; then
            sudo cp "$env_src" "$SYSTEM_CONFIG_DIR/localgo.env"
            sudo chmod 644 "$SYSTEM_CONFIG_DIR/localgo.env"
            ok "Config installed to $SYSTEM_CONFIG_DIR/localgo.env"
        else
            info "Config already exists at $SYSTEM_CONFIG_DIR/localgo.env, skipping"
        fi
    else
        mkdir -p "$USER_CONFIG_DIR"
        if [[ ! -f "$USER_CONFIG_DIR/localgo.env" ]]; then
            cp "$env_src" "$USER_CONFIG_DIR/localgo.env"
            chmod 600 "$USER_CONFIG_DIR/localgo.env"
            ok "Config installed to $USER_CONFIG_DIR/localgo.env"
        else
            info "Config already exists at $USER_CONFIG_DIR/localgo.env, skipping"
        fi
    fi
}

# ── Verify Installation ────────────────────────────────────────────
verify_installation() {
    local bin_path
    [[ "$INSTALL_MODE" == "system" ]] && bin_path="$SYSTEM_BIN_DIR/$BINARY_NAME" || bin_path="$USER_BIN_DIR/$BINARY_NAME"

    if [[ ! -x "$bin_path" ]]; then
        die "Binary not executable at $bin_path"
    fi

    local installed_ver
    installed_ver=$("$bin_path" version 2>/dev/null | grep -o '[0-9]\+\.[0-9]\+\.[0-9]\+' || echo "$VERSION")
    ok "LocalGo v$installed_ver verified at $bin_path"
}

# ── Post-Install Summary ───────────────────────────────────────────
print_summary() {
    local bin_path
    [[ "$INSTALL_MODE" == "system" ]] && bin_path="$SYSTEM_BIN_DIR/$BINARY_NAME" || bin_path="$USER_BIN_DIR/$BINARY_NAME"

    echo
    echo "  ┌─ Installation Complete ──────────────────────────┐"
    printf "  │  LocalGo v%-30s  │\n" "$VERSION"
    echo "  └──────────────────────────────────────────────────┘"
    echo

    info "Binary: $bin_path"

    if [[ "$INSTALL_MODE" == "user" && ":$PATH:" != *":$USER_BIN_DIR:"* ]]; then
        warn "Add $USER_BIN_DIR to your PATH:"
        case "${SHELL:-}" in
            *fish) echo "    fish_add_path $USER_BIN_DIR" ;;
            *zsh)  echo "    echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.zshrc && source ~/.zshrc" ;;
            *)     echo "    echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc && source ~/.bashrc" ;;
        esac
        echo
    fi

    info "Quick start:"
    echo "    $BINARY_NAME help            # Show commands"
    echo "    $BINARY_NAME info            # Device info"
    echo "    $BINARY_NAME discover        # Find peers"
    echo "    $BINARY_NAME send --help     # Send files"
    echo

    if [[ "$INSTALL_SERVICE" == true && "$OS" == "linux" ]]; then
        info "Service management:"
        if [[ "$INSTALL_MODE" == "system" ]]; then
            echo "    sudo systemctl enable --now $BINARY_NAME"
            echo "    sudo journalctl -u $BINARY_NAME -f"
        else
            echo "    systemctl --user enable --now $BINARY_NAME"
            echo "    journalctl --user -u $BINARY_NAME -f"
            echo
            info "To keep service alive after logout:"
            echo "    loginctl enable-linger $USER"
        fi
        echo
    fi
}

# ── Main ───────────────────────────────────────────────────────────
main() {
    echo "  ◆ LocalGo Online Installer"
    echo "    LocalSend v2.1 Protocol CLI"
    echo "    ─────────────────────────────────"
    echo

    # ── Parse args ──
    while [[ $# -gt 0 ]]; do
        case $1 in
            -v|--version)
                PINNED_VERSION="$2"
                shift 2
                ;;
            --mode)
                INSTALL_MODE="$2"
                shift 2
                ;;
            --service)    INSTALL_SERVICE=true;    shift ;;
            --completion) INSTALL_COMPLETION=true; shift ;;
            --config)     INSTALL_CONFIG=true;     shift ;;
            -y|--yes)     ASSUME_YES=true;         shift ;;
            --dry-run)    DRY_RUN=true;             shift ;;
            -h|--help)    usage ;;
            *) die "Unknown option: $1 (use --help for usage)" ;;
        esac
    done

    [[ "$INSTALL_MODE" != "user" && "$INSTALL_MODE" != "system" ]] \
        && die "Invalid mode: $INSTALL_MODE (use user or system)"

    detect_platform
    resolve_version
    print_plan

    [[ "$DRY_RUN" == "true" ]] && { info "Dry run — exiting."; exit 0; }

    # Auto-yes when piped (non-TTY stdin)
    [[ ! -t 0 ]] && ASSUME_YES=true
    confirm_or_skip

    download_and_verify
    install_binary
    verify_installation

    [[ "$INSTALL_COMPLETION" == "true" ]] && install_completions
    [[ "$INSTALL_SERVICE" == "true" ]] && install_service
    [[ "$INSTALL_CONFIG" == "true" ]] && install_config

    print_summary
}

main "$@"
