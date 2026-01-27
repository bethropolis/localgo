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
BINARY_NAME="localgo-cli"
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
INSTALL_SERVICE=false
INSTALL_COMPLETION=true
BUILD_BINARY=true
CREATE_USER=false

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
    --service             Install systemd service (requires system mode)
    --no-completion       Skip bash completion installation
    --no-build            Skip building binary (use existing)
    --create-user         Create localgo system user (system mode only)
    --help                Show this help message

MODES:
    user                  Install for current user only (~/.local/bin)
    system                Install system-wide (/usr/local/bin)

EXAMPLES:
    $0                              # User installation
    $0 --mode system                # System installation
    $0 --mode system --service      # System with systemd service
    $0 --mode system --service --create-user  # Full system setup

REQUIREMENTS:
    - Go 1.19+ (for building)
    - systemd (for service installation)
    - sudo access (for system installation)

EOF
}

# Function to check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."

    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed. Please install Go 1.19+ first."
        exit 1
    fi

    # Check Go version
    GO_VERSION=$(go version | grep -o 'go[0-9]\+\.[0-9]\+' | sed 's/go//')
    MAJOR=$(echo $GO_VERSION | cut -d. -f1)
    MINOR=$(echo $GO_VERSION | cut -d. -f2)

    if [[ $MAJOR -lt 1 ]] || [[ $MAJOR -eq 1 && $MINOR -lt 19 ]]; then
        print_error "Go version $GO_VERSION is too old. Please install Go 1.19+ first."
        exit 1
    fi

    print_success "Go $GO_VERSION found"

    # Check system installation requirements
    if [[ "$INSTALL_MODE" == "system" ]]; then
        if [[ $EUID -eq 0 ]]; then
            print_warning "Running as root. Consider using sudo instead."
        elif ! sudo -n true 2>/dev/null; then
            print_status "System installation requires sudo access. You may be prompted for password."
        fi
    fi

    # Check systemd if service installation requested
    if [[ "$INSTALL_SERVICE" == true ]]; then
        if ! command -v systemctl &> /dev/null; then
            print_error "systemctl not found. systemd is required for service installation."
            exit 1
        fi
        print_success "systemd found"
    fi
}

# Function to build binary
build_binary() {
    if [[ "$BUILD_BINARY" == false ]]; then
        print_status "Skipping binary build"
        return
    fi

    check_prerequisites

    print_status "Building LocalGo binary..."

    cd "$PROJECT_DIR"

    # Get version information
    VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')

    # Construct linker flags string (without -ldflags prefix yet)
    LINKER_FLAGS="-X main.Version=$VERSION -X main.GitCommit=$GIT_COMMIT -X main.BuildDate=$BUILD_DATE"

    # Create temp dir for build
    BUILD_TMP=$(mktemp -d)
    
    # Build binary
    # We must quote "$LINKER_FLAGS" so it is passed as a single argument to -ldflags
    if go build -ldflags "$LINKER_FLAGS" -o "$BUILD_TMP/$BINARY_NAME" ./cmd/localgo-cli; then
        print_success "Binary built successfully"
    else
        print_error "Build failed"
        rm -rf "$BUILD_TMP"
        exit 1
    fi
}

# Function to stop service if running
stop_service_if_running() {
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_status "Stopping running service..."
        sudo systemctl stop "$SERVICE_NAME"
    fi
}

# Function to install binary
install_binary() {
    print_status "Installing binary..."

    # Stop service to avoid "Text file busy"
    if [[ "$INSTALL_MODE" == "system" ]]; then
        stop_service_if_running
    fi

    local bin_dir
    if [[ "$INSTALL_MODE" == "system" ]]; then
        bin_dir="$SYSTEM_BIN_DIR"
        sudo mkdir -p "$bin_dir"
        sudo cp "$BUILD_TMP/$BINARY_NAME" "$bin_dir/"
        sudo chmod +x "$bin_dir/$BINARY_NAME"
    else
        bin_dir="$USER_BIN_DIR"
        mkdir -p "$bin_dir"
        cp "$BUILD_TMP/$BINARY_NAME" "$bin_dir/"
        chmod +x "$bin_dir/$BINARY_NAME"
    fi

    # Clean up temp build
    rm -rf "$BUILD_TMP"

    print_success "Binary installed to $bin_dir/$BINARY_NAME"
}

# Function to create directories
create_directories() {
    print_status "Creating directories..."

    if [[ "$INSTALL_MODE" == "system" ]]; then
        sudo mkdir -p "$SYSTEM_CONFIG_DIR" "$SYSTEM_DATA_DIR" "$SYSTEM_LOG_DIR"

        if [[ "$CREATE_USER" == true ]]; then
            # Create localgo user and group
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

    local config_dir
    local config_file

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
            
            # Replace placeholder path with actual user home
            # Escaping the path for sed
            local download_dir="$HOME/Downloads/LocalGo"
            local escaped_dir=$(echo "$download_dir" | sed 's/\//\\\//g')
            
            if [[ "$OS" == "Darwin" ]]; then
                sed -i '' "s/\/home\/user\/Downloads\/LocalGo/$escaped_dir/g" "$config_file"
            else
                sed -i "s/\/home\/user\/Downloads\/LocalGo/$escaped_dir/g" "$config_file"
            fi
            
            print_success "Configuration installed to $config_file"
        else
            print_status "Configuration already exists at $config_file"
            
            # Hotfix: Check for and fix the bad default path in existing config
            if grep -q "/home/user/Downloads/LocalGo" "$config_file"; then
                print_status "Fixing incorrect download path in existing configuration..."
                local download_dir="$HOME/Downloads/LocalGo"
                local escaped_dir=$(echo "$download_dir" | sed 's/\//\\\//g')
                
                if [[ "$OS" == "Darwin" ]]; then
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
        print_status "Installing user-mode systemd service..."
        
        # Detect correct config directory for systemd (handles Distrobox/HOME mismatch)
        local systemd_config_home="$HOME/.config"
        if command -v systemctl &>/dev/null; then
             local systemd_env
             systemd_env=$(systemctl --user show-environment 2>/dev/null)
             
             local sys_config=$(echo "$systemd_env" | grep "^XDG_CONFIG_HOME=" | cut -d= -f2)
             local sys_home=$(echo "$systemd_env" | grep "^HOME=" | cut -d= -f2)
             
             if [[ -n "$sys_config" ]]; then
                 systemd_config_home="$sys_config"
             elif [[ -n "$sys_home" ]]; then
                 systemd_config_home="$sys_home/.config"
             fi
        fi

        local user_systemd_dir="$systemd_config_home/systemd/user"
        local service_file="$user_systemd_dir/$SERVICE_NAME.service"
        
        mkdir -p "$user_systemd_dir"
        
        # Generate user service file dynamically
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
        chmod 644 "$service_file"
        
        # Force a reload to ensure systemd sees the file in the new location
        systemctl --user daemon-reload
        print_success "User service installed to $service_file"
        print_status "To enable and start the service:"

        echo "    systemctl --user enable $SERVICE_NAME"
        echo "    systemctl --user start $SERVICE_NAME"
        
        # Enable lingering to keep service running after logout
        print_status "Note: Run 'loginctl enable-linger $USER' to keep service running after logout"
    fi
}

# Function to install completion
install_completion() {
    if [[ "$INSTALL_COMPLETION" != true ]]; then
        return
    fi

    echo -e "${BLUE}[INFO]${NC} Detecting shell for completions..."

    # Detect shell based on SHELL environment variable or parent process
    local target_shell=""
    if [[ "$SHELL" == *"fish"* ]]; then
        target_shell="fish"
    elif [[ "$SHELL" == *"bash"* ]]; then
        target_shell="bash"
    else
        # Fallback detection usually defaults to bash compatibility
        target_shell="bash"
    fi

    if [[ "$target_shell" == "fish" ]]; then
        # Install Fish completion (User only)
        local fish_completion_script="$SCRIPT_DIR/fish_completion.fish"
        local fish_user_dir="$HOME/.config/fish/completions"
        
        if [[ -f "$fish_completion_script" ]]; then
            mkdir -p "$fish_user_dir"
            cp "$fish_completion_script" "$fish_user_dir/$BINARY_NAME.fish"
            echo -e "${GREEN}[SUCCESS]${NC} Fish completion installed to $fish_user_dir"
        fi
    elif [[ "$target_shell" == "bash" ]]; then
        # Install Bash completion (User only)
        local completion_script="$SCRIPT_DIR/bash_completion.sh"
        local user_completion_dir="$HOME/.local/share/bash-completion/completions"
        
        mkdir -p "$user_completion_dir"
        cp "$completion_script" "$user_completion_dir/$BINARY_NAME"
        echo -e "${GREEN}[SUCCESS]${NC} Bash completion installed to $user_completion_dir"
        echo -e "${BLUE}[INFO]${NC} Restart your shell or ignore if using a different shell."
    fi
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

        # Test version command
        if "$binary_path" version &>/dev/null; then
            print_success "Version command works"
        else
            print_warning "Version command failed"
        fi

        # Test info command
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
        print_error "Binary is not executable"
        exit 1
    fi
}

# Function to show post-installation instructions
show_post_install() {
    print_success "LocalGo installation completed!"
    echo
    print_status "Next steps:"

    if [[ "$INSTALL_MODE" == "user" ]]; then
        if [[ ":$PATH:" != *":$USER_BIN_DIR:"* ]]; then
            print_warning "Add $USER_BIN_DIR to your PATH:"
            if [[ "$SHELL" == *"fish"* ]]; then
                echo "    fish_add_path $USER_BIN_DIR"
            else
                echo "    echo 'export PATH=\"$USER_BIN_DIR:\$PATH\"' >> ~/.bashrc"
                echo "    source ~/.bashrc"
            fi
        fi
        echo "2. Edit $USER_CONFIG_DIR/localgo.env to customize settings"
        echo "3. Run: $BINARY_NAME info"
        echo "4. Start server: $BINARY_NAME serve"
    else
        echo "1. Edit $SYSTEM_CONFIG_DIR/localgo.env to customize settings"
        if [[ "$INSTALL_SERVICE" == true ]]; then
            echo "2. Enable service: sudo systemctl enable $SERVICE_NAME"
            echo "3. Start service: sudo systemctl start $SERVICE_NAME"
            echo "4. Check status: sudo systemctl status $SERVICE_NAME"
        else
            echo "2. Test installation: $BINARY_NAME info"
            echo "3. Start server: $BINARY_NAME serve"
        fi
    fi

    echo
    print_status "Useful commands:"
    echo "  $BINARY_NAME help           # Show help"
    echo "  $BINARY_NAME info           # Show device info"
    echo "  $BINARY_NAME serve          # Start server"
    echo "  $BINARY_NAME discover       # Find devices"
    echo "  $BINARY_NAME send --help    # Send file help"

    if [[ "$INSTALL_SERVICE" == true ]]; then
        echo
        print_status "Service commands:"
        if [[ "$INSTALL_MODE" == "system" ]]; then
            echo "  sudo systemctl status $SERVICE_NAME    # Check status"
            echo "  sudo journalctl -u $SERVICE_NAME -f    # View logs"
            echo "  sudo systemctl restart $SERVICE_NAME   # Restart"
        else
            echo "  systemctl --user status $SERVICE_NAME    # Check status"
            echo "  journalctl --user -u $SERVICE_NAME -f    # View logs"
            echo "  systemctl --user restart $SERVICE_NAME   # Restart"
        fi
    fi
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

    # Validate mode
    if [[ "$INSTALL_MODE" != "user" && "$INSTALL_MODE" != "system" ]]; then
        print_error "Invalid mode: $INSTALL_MODE. Must be 'user' or 'system'"
        exit 1
    fi

    print_status "Installation mode: $INSTALL_MODE"
    print_status "Install service: $INSTALL_SERVICE"
    print_status "Install completion: $INSTALL_COMPLETION"
    print_status "Create user: $CREATE_USER"
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
