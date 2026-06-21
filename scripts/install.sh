#!/bin/bash

# LocalGo Installation Script
# This script installs LocalGo CLI and optionally sets up systemd service

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BINARY_NAME="localgo"
SERVICE_NAME="localgo"

# Installation paths
SYSTEM_BIN_DIR="/usr/local/bin"
SYSTEM_CONFIG_DIR="/etc/localgo"
SYSTEM_DATA_DIR="/var/lib/localgo"
SYSTEM_SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"
USER_BIN_DIR="$HOME/.local/bin"
USER_CONFIG_DIR="$HOME/.config/localgo"
USER_DATA_DIR="$HOME/Downloads/localgo"

# Default settings
INSTALL_MODE="user"
INSTALL_SERVICE=true   # user service is installed by default
INSTALL_COMPLETION=true
BUILD_BINARY=true
CREATE_USER=false
ASSUME_YES=false

# Path to the built binary (set by build_binary, consumed by install_binary)
BUILT_BINARY=""

# Function to print colored output
print_status() {
    echo -e "  ${BLUE}ℹ${NC}  $1"
}

print_success() {
    echo -e "  ${GREEN}✔${NC}  $1"
}

print_warning() {
    echo -e "  ${YELLOW}⚠${NC}  $1"
}

print_error() {
    echo -e "  ${RED}✖${NC}  $1"
}

# Print a section header with step counter
section() {
    local step_num="$1"
    local step_title="$2"
    echo
    echo -e "  ${BLUE}[${step_num}/7]${NC} ${step_title}"
    echo "  ────────────────────────────────────────────────────"
}

# Function to show usage
show_usage() {
    cat << EOF
LocalGo Installation Script

USAGE:
    $0 [OPTIONS]

OPTIONS:
    --mode MODE           Installation mode: user|system (default: user)
    --service             Install systemd service (default for user mode)
    --no-service          Skip systemd service installation
    --no-completion       Skip shell completion installation
    --no-build            Skip building binary (use existing binary in project bin/ or build/)
    --create-user         Create localgo system user (system mode only)
    -y, --yes             Assume yes; bypass confirmation prompts
    --help                Show this help message

MODES:
    user                  Install for current user only (~/.local/bin).
    system                Install system-wide (/usr/local/bin).

EOF
}

# Resolve systemd config home cleanly
resolve_systemd_config_home() {
    local result="$HOME/.config"
    if command -v systemctl &>/dev/null; then
        local systemd_env
        systemd_env=$(systemctl --user show-environment 2>/dev/null || true)
        local sys_config
        sys_config=$(echo "$systemd_env" | grep "^XDG_CONFIG_HOME=" | cut -d= -f2)
        local sys_home
        sys_home=$(echo "$systemd_env" | grep "^HOME=" | cut -d= -f2)
        if [[ -n "$sys_config" ]]; then
            result="$sys_config"
        elif [[ -n "$sys_home" ]]; then
            result="$sys_home/.config"
        fi
    fi
    echo "$result"
}

# Check for existing installation to notify of upgrades
detect_existing_install() {
    local bin_path=""
    if [[ "$INSTALL_MODE" == "system" ]]; then
        bin_path="$SYSTEM_BIN_DIR/$BINARY_NAME"
    else
        bin_path="$USER_BIN_DIR/$BINARY_NAME"
    fi

    if [[ -x "$bin_path" ]]; then
        local old_ver
        old_ver=$("$bin_path" version 2>/dev/null | grep -o '[0-9]\+\.[0-9]\+\.[0-9]\+' || echo "unknown")
        print_status "Detected existing LocalGo installation (v${old_ver}) at $bin_path"
        print_status "This installation will upgrade it to the latest version."
    fi
}

# Function to check prerequisites
check_prerequisites() {
    if [[ "$BUILD_BINARY" == true ]]; then
        if ! command -v go &> /dev/null; then
            print_error "Go is not installed. Please install Go 1.26+ first."
            exit 1
        fi

        GO_VERSION=$(go version | grep -o 'go[0-9]\+\.[0-9]\+' | sed 's/go//')
        MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
        MINOR=$(echo "$GO_VERSION" | cut -d. -f2)

        if [[ $MAJOR -lt 1 ]] || [[ $MAJOR -eq 1 && $MINOR -lt 26 ]]; then
            print_error "Go version $GO_VERSION is too old. Please install Go 1.26+ first."
            exit 1
        fi

        print_success "Go v$GO_VERSION verified"
    fi

    if [[ "$INSTALL_MODE" == "system" ]]; then
        if [[ $EUID -eq 0 ]]; then
            print_warning "Running as root. Running with sudo is recommended instead."
        elif ! sudo -n true 2>/dev/null; then
            print_status "System installation requires sudo privileges. You may be prompted for your password."
        fi
    fi

    if [[ "$INSTALL_SERVICE" == true ]]; then
        if ! command -v systemctl &> /dev/null; then
            print_error "systemctl not found. systemd is required for service setup."
            exit 1
        fi
        print_success "systemd verified"
    fi
}

# Function to build binary
build_binary() {
    if [[ "$BUILD_BINARY" == false ]]; then
        print_status "Skipping compilation (--no-build)"

        local candidates=(
            "$PROJECT_DIR/bin/$BINARY_NAME"
            "$PROJECT_DIR/build/$BINARY_NAME"
            "$PROJECT_DIR/$BINARY_NAME"
        )
        for candidate in "${candidates[@]}"; do
            if [[ -x "$candidate" ]]; then
                BUILT_BINARY="$candidate"
                print_success "Using pre-compiled binary: $BUILT_BINARY"
                return
            fi
        done

        print_error "No pre-compiled binary found. Build without --no-build."
        exit 1
    fi

    print_status "Compiling LocalGo binary..."

    cd "$PROJECT_DIR"
    local VERSION GIT_COMMIT BUILD_DATE LINKER_FLAGS
    VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')
    LINKER_FLAGS="-X main.Version=$VERSION -X main.GitCommit=$GIT_COMMIT -X main.BuildDate=$BUILD_DATE"

    local build_tmp
    build_tmp=$(mktemp -d)

    if go build -ldflags "$LINKER_FLAGS" -o "$build_tmp/$BINARY_NAME" ./cmd/localgo; then
        BUILT_BINARY="$build_tmp/$BINARY_NAME"
        print_success "Compilation completed successfully (v$VERSION)"
    else
        print_error "Go compilation failed"
        rm -rf "$build_tmp"
        exit 1
    fi
}

# Stop running service before replacing binary
stop_service_if_running() {
    if [[ "$INSTALL_MODE" == "system" ]]; then
        if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            print_status "Stopping active system-wide service..."
            sudo systemctl stop "$SERVICE_NAME"
        fi
    fi

    if command -v systemctl &>/dev/null; then
        if systemctl --user is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            print_status "Stopping active user service..."
            systemctl --user stop "$SERVICE_NAME"
        fi
    fi
}

# Function to install binary
install_binary() {
    stop_service_if_running

    local bin_dir
    if [[ "$INSTALL_MODE" == "system" ]]; then
        bin_dir="$SYSTEM_BIN_DIR"
        sudo mkdir -p "$bin_dir"
        sudo cp "$BUILT_BINARY" "$bin_dir/$BINARY_NAME"
        sudo chmod +x "$bin_dir/$BINARY_NAME"
    else
        bin_dir="$USER_BIN_DIR"
        mkdir -p "$bin_dir"
        cp "$BUILT_BINARY" "$bin_dir/$BINARY_NAME"
        chmod +x "$bin_dir/$BINARY_NAME"
    fi

    # Clean up temp build folder
    local build_dir
    build_dir="$(dirname "$BUILT_BINARY")"
    if [[ "$build_dir" == /tmp/* ]]; then
        rm -rf "$build_dir"
    fi

    print_success "Binary deployed successfully to $bin_dir/$BINARY_NAME"
}

# Function to create directories
create_directories() {
    print_status "Configuring path folders..."

    if [[ "$INSTALL_MODE" == "system" ]]; then
        sudo mkdir -p "$SYSTEM_CONFIG_DIR" "$SYSTEM_DATA_DIR"

        if [[ "$CREATE_USER" == true ]]; then
            if ! id "localgo" &>/dev/null; then
                sudo useradd --system --home "$SYSTEM_DATA_DIR" --shell /bin/false localgo
                print_success "Created system user account 'localgo'"
            fi
            sudo chown -R localgo:localgo "$SYSTEM_DATA_DIR"
            sudo chown root:localgo "$SYSTEM_CONFIG_DIR"
        fi

        sudo chmod 755 "$SYSTEM_DATA_DIR" "$SYSTEM_CONFIG_DIR"
        print_success "System paths successfully prepared"
    else
        mkdir -p "$USER_CONFIG_DIR"
        mkdir -p "$HOME/.local/state/localgo"
        mkdir -p "$HOME/.local/share/localgo"
        mkdir -p "$USER_DATA_DIR"
        print_success "User paths successfully prepared"
    fi
}

# Function to install configuration
install_configuration() {
    local config_file

    if [[ "$INSTALL_MODE" == "system" ]]; then
        config_file="$SYSTEM_CONFIG_DIR/localgo.env"
        if [[ ! -f "$config_file" ]]; then
            sudo cp "$SCRIPT_DIR/localgo.env.example" "$config_file"
            if [[ "$CREATE_USER" == true ]]; then
                sudo chown root:localgo "$config_file"
                sudo chmod 640 "$config_file"
            else
                sudo chmod 644 "$config_file"
            fi
            print_success "Created template configuration: $config_file"
        else
            print_status "Configuration already exists: $config_file (skipping template override)"
        fi
    else
        config_file="$USER_CONFIG_DIR/localgo.env"
        if [[ ! -f "$config_file" ]]; then
            cp "$SCRIPT_DIR/localgo.env.example" "$config_file"
            chmod 600 "$config_file"
            print_success "Created template configuration: $config_file"
        else
            print_status "Configuration already exists: $config_file (skipping template override)"
        fi
    fi
}

# Function to install systemd service
install_service() {
    if [[ "$INSTALL_SERVICE" != true ]]; then
        print_status "Service installation skipped"
        return
    fi

    if [[ "$INSTALL_MODE" == "system" ]]; then
        local service_file="/etc/systemd/system/$SERVICE_NAME.service"
        local tmp_svc
        tmp_svc=$(mktemp)
        cp "$SCRIPT_DIR/localgo.service" "$tmp_svc"

        sed -i "s|ExecStart=.*|ExecStart=$SYSTEM_BIN_DIR/$BINARY_NAME serve --quiet --auto-accept|g" "$tmp_svc"
        sed -i "s|EnvironmentFile=.*|EnvironmentFile=-$SYSTEM_CONFIG_DIR/localgo.env|g" "$tmp_svc"
        sed -i "s|WorkingDirectory=.*|WorkingDirectory=$SYSTEM_DATA_DIR|g" "$tmp_svc"
        sed -i "s|%E/localgo|$SYSTEM_CONFIG_DIR|g" "$tmp_svc"

        if [[ "$CREATE_USER" == false ]]; then
            sed -i "/^User=/d" "$tmp_svc"
            sed -i "/^Group=/d" "$tmp_svc"
        fi

        sudo cp "$tmp_svc" "$service_file"
        sudo chmod 644 "$service_file"
        rm -f "$tmp_svc"

        sudo systemctl daemon-reload
        print_success "System service installed: $service_file"
    else
        local systemd_config_home
        systemd_config_home=$(resolve_systemd_config_home)

        local user_systemd_dir="$systemd_config_home/systemd/user"
        local service_file="$user_systemd_dir/$SERVICE_NAME.service"

        mkdir -p "$user_systemd_dir"

        if [[ -f "$SCRIPT_DIR/localgo-user.service" ]]; then
            cp "$SCRIPT_DIR/localgo-user.service" "$service_file"
        else
            local tmp_svc
            tmp_svc=$(mktemp)
            cp "$SCRIPT_DIR/localgo.service" "$tmp_svc"
            sed -i "s|ExecStart=.*|ExecStart=$USER_BIN_DIR/$BINARY_NAME serve --quiet --auto-accept|g" "$tmp_svc"
            sed -i "/^User=/d" "$tmp_svc"
            sed -i "/^Group=/d" "$tmp_svc"
            cp "$tmp_svc" "$service_file"
            rm -f "$tmp_svc"
        fi
        chmod 644 "$service_file"

        systemctl --user daemon-reload
        print_success "User service installed: $service_file"
    fi
}

# Function to install shell completions for all detected shells
install_completion() {
    if [[ "$INSTALL_COMPLETION" != true ]]; then
        print_status "Completion installation skipped"
        return
    fi

    print_status "Checking for active shells to install completions..."

    if command -v bash &>/dev/null; then
        local completion_script="$SCRIPT_DIR/bash_completion.sh"
        local user_completion_dir="$HOME/.local/share/bash-completion/completions"
        if [[ -f "$completion_script" ]]; then
            mkdir -p "$user_completion_dir"
            cp "$completion_script" "$user_completion_dir/$BINARY_NAME"
            print_success "Bash completion deployed to $user_completion_dir"
        fi
    fi

    if command -v zsh &>/dev/null; then
        local zsh_completion_dir="$HOME/.local/share/zsh/site-functions"
        local completion_script="$SCRIPT_DIR/zsh_completion.zsh"
        if [[ -f "$completion_script" ]]; then
            mkdir -p "$zsh_completion_dir"
            cp "$completion_script" "$zsh_completion_dir/_$BINARY_NAME"
            print_success "Zsh completion deployed to $zsh_completion_dir"
        fi
    fi

    if command -v fish &>/dev/null; then
        local fish_completion_script="$SCRIPT_DIR/fish_completion.fish"
        local fish_user_dir="$HOME/.config/fish/completions"
        if [[ -f "$fish_completion_script" ]]; then
            mkdir -p "$fish_user_dir"
            cp "$fish_completion_script" "$fish_user_dir/$BINARY_NAME.fish"
            print_success "Fish completion deployed to $fish_user_dir"
        fi
    fi
}

# Function to test installation
test_installation() {
    local binary_path
    if [[ "$INSTALL_MODE" == "system" ]]; then
        binary_path="$SYSTEM_BIN_DIR/$BINARY_NAME"
    else
        binary_path="$USER_BIN_DIR/$BINARY_NAME"
    fi

    if [[ -x "$binary_path" ]]; then
        print_success "Installed binary is fully executable"
        if "$binary_path" version &>/dev/null; then
            print_success "LocalGo handshake verified successfully"
        else
            print_warning "Verification returned a structural warning"
        fi
    else
        print_error "Binary is not executable at $binary_path"
        exit 1
    fi
}

# Function to show post-installation instructions
show_post_install() {
    echo
    echo "  ┌─ Installation Complete ────────────────────────────┐"
    echo "  │  LocalGo is ready to run!                          │"
    echo "  └────────────────────────────────────────────────────┘"
    echo

    if [[ "$INSTALL_MODE" == "user" ]]; then
        print_status "Quick Start:"

        if [[ ":$PATH:" != *":$USER_BIN_DIR:"* ]]; then
            print_warning "Add $USER_BIN_DIR to your shell PATH to run localgo directly:"
            case "$SHELL" in
                *fish) echo "    fish_add_path $USER_BIN_DIR" ;;
                *zsh)  echo "    echo 'export PATH=\"$USER_BIN_DIR:\$PATH\"' >> ~/.zshrc && source ~/.zshrc" ;;
                *)     echo "    echo 'export PATH=\"$USER_BIN_DIR:\$PATH\"' >> ~/.bashrc && source ~/.bashrc" ;;
            esac
            echo
        fi

        echo "    1. Edit config at $USER_CONFIG_DIR/localgo.env"
        echo "    2. Verify status:  $BINARY_NAME info"
        echo "    3. Start server:   $BINARY_NAME serve"
        echo

        if [[ "$INSTALL_SERVICE" == true ]]; then
            print_status "Systemd User Service Control:"
            echo "    systemctl --user enable $SERVICE_NAME           # Auto-start on login"
            echo "    systemctl --user start $SERVICE_NAME            # Start background daemon"
            echo "    systemctl --user status $SERVICE_NAME           # Verify service status"
            echo "    journalctl --user -u $SERVICE_NAME -f           # Watch background logs"
            echo
            print_status "To keep user service alive after terminal logout:"
            echo "    loginctl enable-linger $USER"
            echo
        fi

        print_status "Deployed System Paths:"
        echo "    Binary:        $USER_BIN_DIR/$BINARY_NAME"
        echo "    Config:        $USER_CONFIG_DIR/localgo.env"
        if [[ "$INSTALL_SERVICE" == true ]]; then
            echo "    Service:       ~/.config/systemd/user/$SERVICE_NAME.service"
        fi
    else
        print_status "Quick Start:"
        echo "    1. Edit config at $SYSTEM_CONFIG_DIR/localgo.env"
        echo

        if [[ "$INSTALL_SERVICE" == true ]]; then
            print_status "Systemd System Service Control:"
            echo "    sudo systemctl enable $SERVICE_NAME       # Auto-start on boot"
            echo "    sudo systemctl start $SERVICE_NAME        # Start background daemon"
            echo "    sudo systemctl status $SERVICE_NAME      # Verify service status"
            echo "    sudo journalctl -u $SERVICE_NAME -f      # Watch background logs"
            echo
        else
            echo "    2. Verify status: $BINARY_NAME info"
            echo "    3. Start server:  $BINARY_NAME serve"
            echo
        fi

        print_status "Deployed System Paths:"
        echo "    Binary:        $SYSTEM_BIN_DIR/$BINARY_NAME"
        echo "    Config:        $SYSTEM_CONFIG_DIR/localgo.env"
        if [[ "$INSTALL_SERVICE" == true ]]; then
            echo "    Service:       $SYSTEM_SERVICE_FILE"
        fi
    fi

    echo
    print_status "Core Commands Reference:"
    echo "    $BINARY_NAME help            # Show commands help"
    echo "    $BINARY_NAME info            # Print local device identity"
    echo "    $BINARY_NAME discover        # Find peers on your Wi-Fi/LAN"
    echo "    $BINARY_NAME send --help     # Review file sharing options"
    echo
}

# Main installation function
main() {
    echo "  ◆ LocalGo Installer"
    echo "    LocalSend v2.1 Protocol CLI Client"
    echo "    ─────────────────────────────────"
    echo

    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --mode)
                INSTALL_MODE="$2"
                shift 2
                ;;
            --service)
                INSTALL_SERVICE=true
                shift
                ;;
            --no-service)
                INSTALL_SERVICE=false
                shift
                ;;
            --no-completion)
                INSTALL_COMPLETION=false
                shift
                ;;
            --no-build)
                BUILD_BINARY=false
                shift
                ;;
            --create-user)
                CREATE_USER=true
                shift
                ;;
            -y|--yes)
                ASSUME_YES=true
                shift
                ;;
            --help|-h)
                show_usage
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done

    # Validate mode early
    if [[ "$INSTALL_MODE" != "user" && "$INSTALL_MODE" != "system" ]]; then
        print_error "Invalid mode: '$INSTALL_MODE'. Must be 'user' or 'system'."
        exit 1
    fi

    echo "  ┌─ Planned Action Summary ───────────────────────────┐"
    printf "  │  %-22s  %-24s  │\n" "Installation Mode:" "$INSTALL_MODE"
    printf "  │  %-22s  %-24s  │\n" "Compile Binary:" "$BUILD_BINARY"
    printf "  │  %-22s  %-24s  │\n" "Systemd Service:" "$INSTALL_SERVICE"
    printf "  │  %-22s  %-24s  │\n" "Shell Completions:" "$INSTALL_COMPLETION"
    if [[ "$INSTALL_MODE" == "system" ]]; then
        printf "  │  %-22s  %-24s  │\n" "Create System User:" "$CREATE_USER"
    fi
    echo "  └────────────────────────────────────────────────────┘"
    echo

    # Check existing installation
    detect_existing_install

    # Confirm action
    if [[ "$ASSUME_YES" != "true" ]]; then
        echo -n "  Proceed with installation? [y/N]: "
        read -r response
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            print_warning "Installation canceled by user."
            echo
            exit 0
        fi
    fi

    # Run installation steps sequentially with progress counters
    section "1" "Verifying system prerequisites"
    check_prerequisites

    section "2" "Compiling LocalGo static binary"
    build_binary

    section "3" "Creating configuration directories"
    create_directories

    section "4" "Installing executable binary"
    install_binary

    section "5" "Deploying configuration environment"
    install_configuration

    section "6" "Configuring systemd background services"
    install_service

    section "7" "Generating shell auto-completions"
    install_completion

    echo
    echo "  ◆ Post-Installation Check"
    echo "  ─────────────────────────"
    test_installation

    show_post_install
}

# Run main function
main "$@"
