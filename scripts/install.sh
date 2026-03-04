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
SYSTEM_LOG_DIR="/var/log/localgo"
USER_BIN_DIR="$HOME/.local/bin"
USER_CONFIG_DIR="$HOME/.config/localgo"
USER_DATA_DIR="$HOME/.local/share/localgo"

# Default settings
INSTALL_MODE="user"
INSTALL_SERVICE=true   # user service is installed by default
INSTALL_COMPLETION=true
BUILD_BINARY=true
CREATE_USER=false

# Path to the built binary (set by build_binary, consumed by install_binary)
BUILT_BINARY=""

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to show usage
show_usage() {
    cat << EOF
LocalGo Installation Script

USAGE:
    $0 [OPTIONS]

OPTIONS:
    --mode MODE           Installation mode: user|system (default: user)
    --service             Install systemd service (required for system mode; default for user mode)
    --no-service          Skip systemd service installation
    --no-completion       Skip shell completion installation
    --no-build            Skip building binary (use existing binary in project bin/ or build/)
    --create-user         Create localgo system user (system mode only)
    --help                Show this help message

MODES:
    user                  Install for current user only (~/.local/bin).
                          A user systemd service is installed by default.
    system                Install system-wide (/usr/local/bin).
                          Pass --service to also install the system-wide service.

EXAMPLES:
    $0                                        # User install + user service (default)
    $0 --no-service                           # User install without service
    $0 --mode system                          # System install (binary only)
    $0 --mode system --service                # System install + system service
    $0 --mode system --service --create-user  # Full system setup

REQUIREMENTS:
    - Go 1.24+ (for building; not needed with --no-build)
    - systemd (for service installation)
    - sudo access (for system installation)

EOF
}

# Resolve the systemd user config directory, accounting for Distrobox/XDG mismatches
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

# Function to check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."

    if [[ "$BUILD_BINARY" == true ]]; then
        if ! command -v go &> /dev/null; then
            print_error "Go is not installed. Please install Go 1.24+ first."
            exit 1
        fi

        GO_VERSION=$(go version | grep -o 'go[0-9]\+\.[0-9]\+' | sed 's/go//')
        MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
        MINOR=$(echo "$GO_VERSION" | cut -d. -f2)

        if [[ $MAJOR -lt 1 ]] || [[ $MAJOR -eq 1 && $MINOR -lt 24 ]]; then
            print_error "Go version $GO_VERSION is too old. Please install Go 1.24+ first."
            exit 1
        fi

        print_success "Go $GO_VERSION found"
    fi

    if [[ "$INSTALL_MODE" == "system" ]]; then
        if [[ $EUID -eq 0 ]]; then
            print_warning "Running as root. Consider using sudo instead."
        elif ! sudo -n true 2>/dev/null; then
            print_status "System installation requires sudo access. You may be prompted for password."
        fi
    fi

    if [[ "$INSTALL_SERVICE" == true ]]; then
        if ! command -v systemctl &> /dev/null; then
            print_error "systemctl not found. systemd is required for service installation."
            exit 1
        fi
        print_success "systemd found"
    fi
}

# Function to build binary — sets BUILT_BINARY on success
build_binary() {
    if [[ "$BUILD_BINARY" == false ]]; then
        print_status "Skipping binary build (--no-build)"

        # Locate an existing pre-built binary
        local candidates=(
            "$PROJECT_DIR/bin/$BINARY_NAME"
            "$PROJECT_DIR/build/$BINARY_NAME"
            "$PROJECT_DIR/$BINARY_NAME"
        )
        for candidate in "${candidates[@]}"; do
            if [[ -x "$candidate" ]]; then
                BUILT_BINARY="$candidate"
                print_success "Using existing binary: $BUILT_BINARY"
                return
            fi
        done

        print_error "No pre-built binary found. Expected one of:"
        for candidate in "${candidates[@]}"; do
            echo "    $candidate"
        done
        print_error "Run without --no-build, or place the binary in one of the paths above."
        exit 1
    fi

    print_status "Building LocalGo binary..."

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
        print_success "Binary built successfully ($VERSION)"
    else
        print_error "Build failed"
        rm -rf "$build_tmp"
        exit 1
    fi
}

# Function to stop any running localgo service (user and/or system) before replacing the binary
stop_service_if_running() {
    if [[ "$INSTALL_MODE" == "system" ]]; then
        if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            print_status "Stopping running system service..."
            sudo systemctl stop "$SERVICE_NAME"
        fi
    fi

    if command -v systemctl &>/dev/null; then
        if systemctl --user is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            print_status "Stopping running user service..."
            systemctl --user stop "$SERVICE_NAME"
        fi
    fi
}

# Function to install binary
install_binary() {
    print_status "Installing binary..."

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

    # Clean up temp build dir if it was a mktemp directory
    local build_dir
    build_dir="$(dirname "$BUILT_BINARY")"
    if [[ "$build_dir" == /tmp/* ]]; then
        rm -rf "$build_dir"
    fi

    print_success "Binary installed to $bin_dir/$BINARY_NAME"
}

# Function to create directories
create_directories() {
    print_status "Creating directories..."

    if [[ "$INSTALL_MODE" == "system" ]]; then
        sudo mkdir -p "$SYSTEM_CONFIG_DIR" "$SYSTEM_DATA_DIR" "$SYSTEM_LOG_DIR"

        if [[ "$CREATE_USER" == true ]]; then
            if ! id "localgo" &>/dev/null; then
                print_status "Creating localgo user..."
                sudo useradd --system --home "$SYSTEM_DATA_DIR" --shell /bin/false localgo
                print_success "Created localgo user"
            else
                print_status "localgo user already exists"
            fi

            sudo chown -R localgo:localgo "$SYSTEM_DATA_DIR" "$SYSTEM_LOG_DIR"
            sudo chown root:localgo "$SYSTEM_CONFIG_DIR"
        fi

        sudo chmod 755 "$SYSTEM_DATA_DIR" "$SYSTEM_LOG_DIR" "$SYSTEM_CONFIG_DIR"
        print_success "System directories created"
    else
        mkdir -p "$USER_CONFIG_DIR" "$USER_DATA_DIR"
        print_success "User directories created"
    fi
}

# Function to install configuration
install_configuration() {
    print_status "Installing configuration..."

    local config_dir config_file

    if [[ "$INSTALL_MODE" == "system" ]]; then
        config_dir="$SYSTEM_CONFIG_DIR"
        config_file="$config_dir/localgo.env"

        if [[ ! -f "$config_file" ]]; then
            sudo cp "$SCRIPT_DIR/localgo.env.example" "$config_file"
            if [[ "$CREATE_USER" == true ]]; then
                sudo chown root:localgo "$config_file"
                sudo chmod 640 "$config_file"
            else
                sudo chmod 644 "$config_file"
            fi
            print_success "Configuration installed to $config_file"
        else
            print_status "Configuration already exists at $config_file"
        fi
    else
        config_dir="$USER_CONFIG_DIR"
        config_file="$config_dir/localgo.env"

        if [[ ! -f "$config_file" ]]; then
            cp "$SCRIPT_DIR/localgo.env.example" "$config_file"
            chmod 600 "$config_file"

            local download_dir="$HOME/Downloads/LocalGo"
            local escaped_dir
            escaped_dir=$(echo "$download_dir" | sed 's/\//\\\//g')

            if [[ "$(uname)" == "Darwin" ]]; then
                sed -i '' "s/\/home\/user\/Downloads\/LocalGo/$escaped_dir/g" "$config_file"
            else
                sed -i "s/\/home\/user\/Downloads\/LocalGo/$escaped_dir/g" "$config_file"
            fi

            print_success "Configuration installed to $config_file"
        else
            print_status "Configuration already exists at $config_file"

            if grep -q "/home/user/Downloads/LocalGo" "$config_file"; then
                print_status "Fixing incorrect download path in existing configuration..."
                local download_dir="$HOME/Downloads/LocalGo"
                local escaped_dir
                escaped_dir=$(echo "$download_dir" | sed 's/\//\\\//g')

                if [[ "$(uname)" == "Darwin" ]]; then
                    sed -i '' "s/\/home\/user\/Downloads\/LocalGo/$escaped_dir/g" "$config_file"
                else
                    sed -i "s/\/home\/user\/Downloads\/LocalGo/$escaped_dir/g" "$config_file"
                fi
                print_success "Configuration updated with correct path"
            fi
        fi
    fi

    print_status "Edit $config_file to customize settings"
}

# Function to install systemd service
install_service() {
    if [[ "$INSTALL_SERVICE" != true ]]; then
        return
    fi

    if [[ "$INSTALL_MODE" == "system" ]]; then
        print_status "Installing system-wide systemd service..."
        local service_file="/etc/systemd/system/$SERVICE_NAME.service"
        sudo cp "$SCRIPT_DIR/localgo.service" "$service_file"
        sudo chmod 644 "$service_file"
        sudo systemctl daemon-reload
        print_success "Service installed to $service_file"
        print_status "To enable and start the service:"
        echo "    sudo systemctl enable $SERVICE_NAME"
        echo "    sudo systemctl start $SERVICE_NAME"
    else
        print_status "Installing user systemd service..."

        local systemd_config_home
        systemd_config_home=$(resolve_systemd_config_home)

        local user_systemd_dir="$systemd_config_home/systemd/user"
        local service_file="$user_systemd_dir/$SERVICE_NAME.service"

        mkdir -p "$user_systemd_dir"

        if [[ -f "$SCRIPT_DIR/localgo-user.service" ]]; then
            cp "$SCRIPT_DIR/localgo-user.service" "$service_file"
        else
            cat > "$service_file" <<EOF
[Unit]
Description=LocalGo (User Service)
Documentation=https://github.com/bethropolis/localgo
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$USER_BIN_DIR/$BINARY_NAME serve --quiet
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=localgo-user

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
RestrictRealtime=true

# Environment configuration
EnvironmentFile=-$USER_CONFIG_DIR/localgo.env
WorkingDirectory=$USER_DATA_DIR

[Install]
WantedBy=default.target
EOF
        fi
        chmod 644 "$service_file"

        systemctl --user daemon-reload
        print_success "User service installed to $service_file"
        print_status "To enable and start the service:"
        echo "    systemctl --user enable $SERVICE_NAME"
        echo "    systemctl --user start $SERVICE_NAME"
    fi
}

# Function to install shell completions
install_completion() {
    if [[ "$INSTALL_COMPLETION" != true ]]; then
        return
    fi

    print_status "Detecting shell for completions..."

    local target_shell=""
    case "$SHELL" in
        *fish) target_shell="fish" ;;
        *zsh)  target_shell="zsh"  ;;
        *)     target_shell="bash" ;;
    esac

    case "$target_shell" in
        fish)
            local fish_completion_script="$SCRIPT_DIR/fish_completion.fish"
            local fish_user_dir="$HOME/.config/fish/completions"
            if [[ -f "$fish_completion_script" ]]; then
                mkdir -p "$fish_user_dir"
                cp "$fish_completion_script" "$fish_user_dir/$BINARY_NAME.fish"
                print_success "Fish completion installed to $fish_user_dir"
            else
                print_warning "Fish completion script not found: $fish_completion_script"
            fi
            ;;
        zsh)
            local zsh_completion_dir="$HOME/.local/share/zsh/site-functions"
            local completion_script="$SCRIPT_DIR/bash_completion.sh"
            if [[ -f "$completion_script" ]]; then
                mkdir -p "$zsh_completion_dir"
                cp "$completion_script" "$zsh_completion_dir/_$BINARY_NAME"
                print_success "Zsh completion installed to $zsh_completion_dir"
                print_status "Add '$zsh_completion_dir' to your fpath if not already present."
            else
                print_warning "Completion script not found: $completion_script"
            fi
            ;;
        bash)
            local completion_script="$SCRIPT_DIR/bash_completion.sh"
            local user_completion_dir="$HOME/.local/share/bash-completion/completions"
            if [[ -f "$completion_script" ]]; then
                mkdir -p "$user_completion_dir"
                cp "$completion_script" "$user_completion_dir/$BINARY_NAME"
                print_success "Bash completion installed to $user_completion_dir"
                print_status "Restart your shell or run: source $user_completion_dir/$BINARY_NAME"
            else
                print_warning "Completion script not found: $completion_script"
            fi
            ;;
    esac
}

# Function to test installation
test_installation() {
    print_status "Testing installation..."

    local binary_path
    if [[ "$INSTALL_MODE" == "system" ]]; then
        binary_path="$SYSTEM_BIN_DIR/$BINARY_NAME"
    else
        binary_path="$USER_BIN_DIR/$BINARY_NAME"
    fi

    if [[ -x "$binary_path" ]]; then
        print_success "Binary is executable"

        if "$binary_path" version &>/dev/null; then
            print_success "Version command works"
        else
            print_warning "Version command failed"
        fi

        if [[ "$INSTALL_MODE" == "system" && "$CREATE_USER" == true ]]; then
            if sudo -u localgo "$binary_path" info &>/dev/null; then
                print_success "Info command works for localgo user"
            else
                print_warning "Info command failed for localgo user"
            fi
        else
            if "$binary_path" info &>/dev/null; then
                print_success "Info command works"
            else
                print_warning "Info command failed"
            fi
        fi
    else
        print_error "Binary is not executable at $binary_path"
        exit 1
    fi
}

# Function to show post-installation instructions
show_post_install() {
    print_success "LocalGo installation completed!"
    echo

    if [[ "$INSTALL_MODE" == "user" ]]; then
        print_status "Next steps:"

        if [[ ":$PATH:" != *":$USER_BIN_DIR:"* ]]; then
            print_warning "Add $USER_BIN_DIR to your PATH:"
            case "$SHELL" in
                *fish)
                    echo "    fish_add_path $USER_BIN_DIR"
                    ;;
                *zsh)
                    echo "    echo 'export PATH=\"$USER_BIN_DIR:\$PATH\"' >> ~/.zshrc"
                    echo "    source ~/.zshrc"
                    ;;
                *)
                    echo "    echo 'export PATH=\"$USER_BIN_DIR:\$PATH\"' >> ~/.bashrc"
                    echo "    source ~/.bashrc"
                    ;;
            esac
        fi

        echo "1. Edit $USER_CONFIG_DIR/localgo.env to customize settings"
        echo "2. Run: $BINARY_NAME info"
        echo "3. Start server: $BINARY_NAME serve"

        if [[ "$INSTALL_SERVICE" == true ]]; then
            echo
            print_status "User service commands:"
            echo "  systemctl --user enable $SERVICE_NAME   # Enable on login"
            echo "  systemctl --user start $SERVICE_NAME    # Start now"
            echo "  systemctl --user status $SERVICE_NAME   # Check status"
            echo "  journalctl --user -u $SERVICE_NAME -f   # View logs"
            echo
            print_status "To keep the service running after logout:"
            echo "    loginctl enable-linger $USER"
        fi
    else
        print_status "Next steps:"
        echo "1. Edit $SYSTEM_CONFIG_DIR/localgo.env to customize settings"

        if [[ "$INSTALL_SERVICE" == true ]]; then
            echo "2. Enable service: sudo systemctl enable $SERVICE_NAME"
            echo "3. Start service:  sudo systemctl start $SERVICE_NAME"
            echo "4. Check status:   sudo systemctl status $SERVICE_NAME"
            echo
            print_status "Service log:"
            echo "  sudo journalctl -u $SERVICE_NAME -f"
        else
            echo "2. Test installation: $BINARY_NAME info"
            echo "3. Start server:      $BINARY_NAME serve"
        fi
    fi

    echo
    print_status "Useful commands:"
    echo "  $BINARY_NAME help           # Show help"
    echo "  $BINARY_NAME info           # Show device info"
    echo "  $BINARY_NAME serve          # Start server"
    echo "  $BINARY_NAME discover       # Find devices"
    echo "  $BINARY_NAME send --help    # Send file help"
}

# Main installation function
main() {
    echo "LocalGo Installation Script"
    echo "==========================="
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

    print_status "Installation mode: $INSTALL_MODE"
    print_status "Install service:   $INSTALL_SERVICE"
    print_status "Install completion: $INSTALL_COMPLETION"
    if [[ "$INSTALL_MODE" == "system" ]]; then
        print_status "Create system user: $CREATE_USER"
    fi
    echo

    # Run installation steps
    check_prerequisites
    build_binary
    create_directories
    install_binary
    install_configuration
    install_service
    install_completion
    test_installation
    show_post_install
}

# Run main function
main "$@"
