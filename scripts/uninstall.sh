#!/bin/bash

# LocalGo Uninstall Script
# This script removes LocalGo CLI and associated components

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="localgo-cli"
SERVICE_NAME="localgo"

# Installation paths
SYSTEM_BIN_DIR="/usr/local/bin"
SYSTEM_CONFIG_DIR="/etc/localgo"
SYSTEM_DATA_DIR="/var/lib/localgo"
SYSTEM_LOG_DIR="/var/log/localgo"
SYSTEM_SERVICE_FILE="/etc/systemd/system/localgo.service"
SYSTEM_COMPLETION_DIR="/etc/bash_completion.d"

USER_BIN_DIR="$HOME/.local/bin"
USER_CONFIG_DIR="$HOME/.config/localgo"
USER_DATA_DIR="$HOME/.local/share/localgo"
USER_COMPLETION_DIR="$HOME/.local/share/bash-completion/completions"

# Default settings
REMOVE_CONFIG=false
REMOVE_DATA=false
REMOVE_USER=false
FORCE=false
DRY_RUN=false
INTERACTIVE=true

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

print_action() {
    if [[ "$DRY_RUN" == true ]]; then
        echo -e "${YELLOW}[DRY RUN]${NC} Would: $1"
    else
        echo -e "${GREEN}[REMOVING]${NC} $1"
    fi
}

# Function to show usage
show_usage() {
    cat << EOF
LocalGo Uninstall Script

USAGE:
    $0 [OPTIONS]

OPTIONS:
    --remove-config       Remove configuration files
    --remove-data         Remove data directories and downloaded files
    --remove-user         Remove localgo system user (system mode only)
    --force               Skip confirmation prompts
    --dry-run             Show what would be removed without actually removing
    --non-interactive     Don't ask for confirmations (use with --force)
    --help                Show this help message

EXAMPLES:
    $0                              # Basic uninstall (keeps config and data)
    $0 --remove-config              # Remove including configuration
    $0 --remove-config --remove-data # Complete removal
    $0 --dry-run                    # Preview what would be removed
    $0 --force --remove-config --remove-data # Force complete removal

SAFETY:
    By default, this script preserves:
    - Configuration files
    - Downloaded files and data directories
    - System user account

    Use the appropriate flags to remove these items.

EOF
}

# Function to ask for confirmation
confirm_action() {
    if [[ "$FORCE" == true ]] || [[ "$INTERACTIVE" == false ]]; then
        return 0
    fi

    local message="$1"
    echo -e "${YELLOW}$message${NC}"
    read -p "Continue? [y/N] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        return 0
    else
        return 1
    fi
}

# Function to safely remove file
safe_remove_file() {
    local file="$1"
    local use_sudo="$2"

    if [[ -f "$file" ]]; then
        print_action "Remove file: $file"
        if [[ "$DRY_RUN" != true ]]; then
            if [[ "$use_sudo" == true ]]; then
                sudo rm -f "$file"
            else
                rm -f "$file"
            fi
        fi
        return 0
    else
        return 1
    fi
}

# Function to safely remove directory
safe_remove_dir() {
    local dir="$1"
    local use_sudo="$2"
    local recursive="$3"

    if [[ -d "$dir" ]]; then
        print_action "Remove directory: $dir"
        if [[ "$DRY_RUN" != true ]]; then
            if [[ "$recursive" == true ]]; then
                if [[ "$use_sudo" == true ]]; then
                    sudo rm -rf "$dir"
                else
                    rm -rf "$dir"
                fi
            else
                if [[ "$use_sudo" == true ]]; then
                    sudo rmdir "$dir" 2>/dev/null || true
                else
                    rmdir "$dir" 2>/dev/null || true
                fi
            fi
        fi
        return 0
    else
        return 1
    fi
}

# Function to detect installation type
detect_installation() {
    local system_binary="$SYSTEM_BIN_DIR/$BINARY_NAME"
    local user_binary="$USER_BIN_DIR/$BINARY_NAME"
    local service_file="$SYSTEM_SERVICE_FILE"

    local found_system=false
    local found_user=false
    local found_service=false

    if [[ -f "$system_binary" ]]; then
        found_system=true
    fi

    if [[ -f "$user_binary" ]]; then
        found_user=true
    fi

    if [[ -f "$service_file" ]]; then
        found_service=true
    fi

    echo "system:$found_system user:$found_user service:$found_service"
}

# Function to stop and disable service
remove_service() {
    if [[ -f "$SYSTEM_SERVICE_FILE" ]]; then
        print_status "Found systemd service, removing..."

        if [[ "$DRY_RUN" != true ]]; then
            # Stop service if running
            if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
                print_action "Stop service: $SERVICE_NAME"
                sudo systemctl stop "$SERVICE_NAME"
            fi

            # Disable service if enabled
            if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
                print_action "Disable service: $SERVICE_NAME"
                sudo systemctl disable "$SERVICE_NAME"
            fi
        fi

        # Remove service file
        safe_remove_file "$SYSTEM_SERVICE_FILE" true

        if [[ "$DRY_RUN" != true ]]; then
            # Reload systemd
            print_action "Reload systemd daemon"
            sudo systemctl daemon-reload
        fi

        print_success "Service removed"
    fi
}

# Function to remove binaries
remove_binaries() {
    local removed=false

    # Remove system binary
    if safe_remove_file "$SYSTEM_BIN_DIR/$BINARY_NAME" true; then
        print_success "Removed system binary"
        removed=true
    fi

    # Remove user binary
    if safe_remove_file "$USER_BIN_DIR/$BINARY_NAME" false; then
        print_success "Removed user binary"
        removed=true
    fi

    if [[ "$removed" != true ]]; then
        print_warning "No binaries found to remove"
    fi
}

# Function to remove bash completion
remove_completion() {
    local removed=false

    # Remove system completion
    if safe_remove_file "$SYSTEM_COMPLETION_DIR/$BINARY_NAME" true; then
        print_success "Removed system bash completion"
        removed=true
    fi

    # Remove user completion
    if safe_remove_file "$USER_COMPLETION_DIR/$BINARY_NAME" false; then
        print_success "Removed user bash completion"
        removed=true
    fi

    if [[ "$removed" != true ]]; then
        print_status "No bash completion found to remove"
    fi
}

# Function to remove configuration
remove_configuration() {
    if [[ "$REMOVE_CONFIG" != true ]]; then
        print_status "Skipping configuration removal (use --remove-config to remove)"
        return
    fi

    local removed=false

    # Remove system config
    if [[ -d "$SYSTEM_CONFIG_DIR" ]]; then
        if confirm_action "Remove system configuration directory: $SYSTEM_CONFIG_DIR"; then
            safe_remove_dir "$SYSTEM_CONFIG_DIR" true true
            print_success "Removed system configuration"
            removed=true
        fi
    fi

    # Remove user config
    if [[ -d "$USER_CONFIG_DIR" ]]; then
        if confirm_action "Remove user configuration directory: $USER_CONFIG_DIR"; then
            safe_remove_dir "$USER_CONFIG_DIR" false true
            print_success "Removed user configuration"
            removed=true
        fi
    fi

    if [[ "$removed" != true ]]; then
        print_status "No configuration directories found"
    fi
}

# Function to remove data directories
remove_data() {
    if [[ "$REMOVE_DATA" != true ]]; then
        print_status "Skipping data removal (use --remove-data to remove)"
        return
    fi

    local removed=false

    # Remove system data
    if [[ -d "$SYSTEM_DATA_DIR" ]]; then
        if confirm_action "Remove system data directory (including downloads): $SYSTEM_DATA_DIR"; then
            safe_remove_dir "$SYSTEM_DATA_DIR" true true
            print_success "Removed system data directory"
            removed=true
        fi
    fi

    # Remove system logs
    if [[ -d "$SYSTEM_LOG_DIR" ]]; then
        if confirm_action "Remove system log directory: $SYSTEM_LOG_DIR"; then
            safe_remove_dir "$SYSTEM_LOG_DIR" true true
            print_success "Removed system log directory"
            removed=true
        fi
    fi

    # Remove user data
    if [[ -d "$USER_DATA_DIR" ]]; then
        if confirm_action "Remove user data directory: $USER_DATA_DIR"; then
            safe_remove_dir "$USER_DATA_DIR" false true
            print_success "Removed user data directory"
            removed=true
        fi
    fi

    # Remove security directory (both system and user locations)
    local security_dirs=("/tmp/.localgo_security" "$HOME/.localgo_security" "/var/lib/localgo/.localgo_security")
    for dir in "${security_dirs[@]}"; do
        if [[ -d "$dir" ]]; then
            if confirm_action "Remove security directory: $dir"; then
                if [[ "$dir" == "/var/lib/localgo/.localgo_security" ]]; then
                    safe_remove_dir "$dir" true true
                else
                    safe_remove_dir "$dir" false true
                fi
                print_success "Removed security directory: $dir"
                removed=true
            fi
        fi
    done

    if [[ "$removed" != true ]]; then
        print_status "No data directories found"
    fi
}

# Function to remove system user
remove_system_user() {
    if [[ "$REMOVE_USER" != true ]]; then
        print_status "Skipping user removal (use --remove-user to remove)"
        return
    fi

    if id "localgo" &>/dev/null; then
        if confirm_action "Remove system user 'localgo' and group"; then
            print_action "Remove user: localgo"
            if [[ "$DRY_RUN" != true ]]; then
                sudo userdel localgo 2>/dev/null || true
                # Remove group if it exists and is empty
                sudo groupdel localgo 2>/dev/null || true
            fi
            print_success "Removed system user and group"
        fi
    else
        print_status "System user 'localgo' not found"
    fi
}

# Function to show what would be removed
show_removal_plan() {
    local detection_result
    detection_result=$(detect_installation)

    local system_install=$(echo "$detection_result" | cut -d: -f2)
    local user_install=$(echo "$detection_result" | cut -d: -f3)
    local service_install=$(echo "$detection_result" | cut -d: -f4)

    echo
    print_status "LocalGo Uninstall Plan"
    echo "======================="
    echo

    # Show what will be removed
    echo "Components to remove:"

    if [[ "$system_install" == true ]]; then
        echo "  ✓ System binary: $SYSTEM_BIN_DIR/$BINARY_NAME"
    fi

    if [[ "$user_install" == true ]]; then
        echo "  ✓ User binary: $USER_BIN_DIR/$BINARY_NAME"
    fi

    if [[ "$service_install" == true ]]; then
        echo "  ✓ Systemd service: $SYSTEM_SERVICE_FILE"
    fi

    echo "  ✓ Bash completion files"

    if [[ "$REMOVE_CONFIG" == true ]]; then
        echo "  ✓ Configuration directories"
    else
        echo "  ✗ Configuration directories (preserved, use --remove-config)"
    fi

    if [[ "$REMOVE_DATA" == true ]]; then
        echo "  ✓ Data directories and downloads"
        echo "  ✓ Security certificates"
    else
        echo "  ✗ Data directories and downloads (preserved, use --remove-data)"
        echo "  ✗ Security certificates (preserved, use --remove-data)"
    fi

    if [[ "$REMOVE_USER" == true ]]; then
        echo "  ✓ System user 'localgo'"
    else
        echo "  ✗ System user 'localgo' (preserved, use --remove-user)"
    fi

    echo

    # Show warnings
    if [[ "$REMOVE_DATA" == true ]]; then
        print_warning "Data removal will delete all downloaded files!"
    fi

    if [[ "$REMOVE_CONFIG" == true ]]; then
        print_warning "Configuration removal will delete all settings!"
    fi

    echo
}

# Function to verify removal
verify_removal() {
    print_status "Verifying removal..."

    local issues_found=false

    # Check binaries
    if [[ -f "$SYSTEM_BIN_DIR/$BINARY_NAME" ]]; then
        print_warning "System binary still exists: $SYSTEM_BIN_DIR/$BINARY_NAME"
        issues_found=true
    fi

    if [[ -f "$USER_BIN_DIR/$BINARY_NAME" ]]; then
        print_warning "User binary still exists: $USER_BIN_DIR/$BINARY_NAME"
        issues_found=true
    fi

    # Check service
    if [[ -f "$SYSTEM_SERVICE_FILE" ]]; then
        print_warning "Service file still exists: $SYSTEM_SERVICE_FILE"
        issues_found=true
    fi

    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        print_warning "Service is still running: $SERVICE_NAME"
        issues_found=true
    fi

    if [[ "$issues_found" != true ]]; then
        print_success "Removal verification passed"
    else
        print_warning "Some components may not have been completely removed"
    fi
}

# Main uninstall function
main() {
    echo "LocalGo Uninstall Script"
    echo "========================"
    echo

    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --remove-config)
                REMOVE_CONFIG=true
                shift
                ;;
            --remove-data)
                REMOVE_DATA=true
                shift
                ;;
            --remove-user)
                REMOVE_USER=true
                shift
                ;;
            --force)
                FORCE=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --non-interactive)
                INTERACTIVE=false
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

    # Show removal plan
    show_removal_plan

    # Final confirmation
    if [[ "$DRY_RUN" != true ]]; then
        if ! confirm_action "Proceed with LocalGo removal?"; then
            print_status "Uninstall cancelled"
            exit 0
        fi
        echo
    fi

    # Perform removal
    print_status "Starting LocalGo removal..."
    echo

    # Remove service first (stops any running processes)
    remove_service

    # Remove binaries
    remove_binaries

    # Remove completion
    remove_completion

    # Remove configuration (if requested)
    remove_configuration

    # Remove data (if requested)
    remove_data

    # Remove system user (if requested)
    remove_system_user

    # Verify removal
    if [[ "$DRY_RUN" != true ]]; then
        verify_removal
    fi

    echo
    if [[ "$DRY_RUN" == true ]]; then
        print_success "Dry run completed - no changes made"
        print_status "Run without --dry-run to perform actual removal"
    else
        print_success "LocalGo uninstall completed!"

        if [[ "$REMOVE_CONFIG" != true ]] || [[ "$REMOVE_DATA" != true ]]; then
            echo
            print_status "Note: Some files were preserved"
            if [[ "$REMOVE_CONFIG" != true ]]; then
                echo "  - Configuration files (use --remove-config to remove)"
            fi
            if [[ "$REMOVE_DATA" != true ]]; then
                echo "  - Data directories and downloads (use --remove-data to remove)"
            fi
            echo
            print_status "To completely remove LocalGo, run:"
            echo "  $0 --remove-config --remove-data --remove-user"
        fi
    fi
}

# Run main function
main "$@"
