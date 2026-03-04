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
BINARY_NAME="localgo"
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
UNINSTALL_MODE="all"   # all | user | system

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
    --mode MODE           Scope removal: all|user|system (default: all)
    --remove-config       Remove configuration files
    --remove-data         Remove data directories and downloaded files
    --remove-user         Remove localgo system user (system mode only)
    --force               Skip confirmation prompts
    --dry-run             Show what would be removed without actually removing
    --non-interactive     Don't ask for confirmations (use with --force)
    --help                Show this help message

MODES:
    all                   Remove both user and system artifacts (default)
    user                  Remove only user-mode artifacts (~/.local/bin, user service, etc.)
    system                Remove only system-wide artifacts (/usr/local/bin, system service, etc.)

EXAMPLES:
    $0                                          # Basic uninstall (keeps config and data)
    $0 --mode user                              # Remove only user installation
    $0 --mode system                            # Remove only system installation
    $0 --remove-config                          # Remove including configuration
    $0 --remove-config --remove-data            # Complete removal
    $0 --dry-run                                # Preview what would be removed
    $0 --force --remove-config --remove-data    # Force complete removal

SAFETY:
    By default, this script preserves:
    - Configuration files
    - Downloaded files and data directories
    - System user account

    Use the appropriate flags to remove these items.

EOF
}

# Resolve the systemd user config directory, accounting for Distrobox/XDG mismatches.
# Mirrors the same logic used in install.sh.
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

# Cache the resolved user service file path (computed once)
user_service_file() {
    local systemd_config_home
    systemd_config_home=$(resolve_systemd_config_home)
    echo "$systemd_config_home/systemd/user/$SERVICE_NAME.service"
}

# Function to ask for confirmation
# Returns 0 (yes) or 1 (no). Does NOT rely on set -e propagating the non-zero return.
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

# Function to safely remove a file
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

# Function to safely remove a directory
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

# Detect what is currently installed; outputs structured info for show_removal_plan
detect_installation() {
    local user_svc
    user_svc=$(user_service_file)

    local found_system_binary=false
    local found_user_binary=false
    local found_system_service=false
    local found_user_service=false

    [[ -f "$SYSTEM_BIN_DIR/$BINARY_NAME" ]] && found_system_binary=true
    [[ -f "$USER_BIN_DIR/$BINARY_NAME" ]]   && found_user_binary=true
    [[ -f "$SYSTEM_SERVICE_FILE" ]]          && found_system_service=true
    [[ -f "$user_svc" ]]                     && found_user_service=true

    echo "system_binary:$found_system_binary user_binary:$found_user_binary system_service:$found_system_service user_service:$found_user_service"
}

# Function to stop and disable services
remove_service() {
    local user_svc
    user_svc=$(user_service_file)

    # System service
    if [[ "$UNINSTALL_MODE" != "user" ]]; then
        if [[ -f "$SYSTEM_SERVICE_FILE" ]] || systemctl list-unit-files 2>/dev/null | grep -q "^$SERVICE_NAME.service"; then
            print_status "Found system-wide service, removing..."
            if [[ "$DRY_RUN" != true ]]; then
                systemctl is-active  --quiet "$SERVICE_NAME" 2>/dev/null && sudo systemctl stop    "$SERVICE_NAME" || true
                systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null && sudo systemctl disable "$SERVICE_NAME" || true
            fi
            safe_remove_file "$SYSTEM_SERVICE_FILE" true
            if [[ "$DRY_RUN" != true ]]; then
                sudo systemctl daemon-reload
                sudo systemctl reset-failed 2>/dev/null || true
            fi
            print_success "System service removed"
        fi
    fi

    # User service
    if [[ "$UNINSTALL_MODE" != "system" ]]; then
        if [[ -f "$user_svc" ]] || systemctl --user list-unit-files 2>/dev/null | grep -q "^$SERVICE_NAME.service"; then
            print_status "Found user service, removing..."
            if [[ "$DRY_RUN" != true ]]; then
                systemctl --user is-active  --quiet "$SERVICE_NAME" 2>/dev/null && systemctl --user stop    "$SERVICE_NAME" || true
                systemctl --user is-enabled --quiet "$SERVICE_NAME" 2>/dev/null && systemctl --user disable "$SERVICE_NAME" || true
            fi
            if ! safe_remove_file "$user_svc" false; then
                # Fallback to hardcoded default in case XDG detection mismatched at install time
                safe_remove_file "$HOME/.config/systemd/user/$SERVICE_NAME.service" false || true
            fi
            if [[ "$DRY_RUN" != true ]]; then
                systemctl --user daemon-reload
                systemctl --user reset-failed 2>/dev/null || true
            fi
            print_success "User service removed"
            print_status "If you enabled lingering, you can clean it up with:"
            echo "    loginctl disable-linger $USER"
        fi
    fi
}

# Function to remove binaries
remove_binaries() {
    local removed=false

    if [[ "$UNINSTALL_MODE" != "user" ]]; then
        if safe_remove_file "$SYSTEM_BIN_DIR/$BINARY_NAME" true; then
            print_success "Removed system binary"
            removed=true
        fi
    fi

    if [[ "$UNINSTALL_MODE" != "system" ]]; then
        if safe_remove_file "$USER_BIN_DIR/$BINARY_NAME" false; then
            print_success "Removed user binary"
            removed=true
        fi
    fi

    if [[ "$removed" != true ]]; then
        print_warning "No binaries found to remove"
    fi
}

# Function to remove shell completions
remove_completion() {
    local removed=false

    if [[ "$UNINSTALL_MODE" != "user" ]]; then
        # System bash completion (legacy cleanup)
        if safe_remove_file "$SYSTEM_COMPLETION_DIR/$BINARY_NAME" true; then
            print_success "Removed system bash completion"
            removed=true
        fi

        # System fish completion (legacy cleanup)
        local fish_sys_completion="/usr/share/fish/vendor_completions.d/$BINARY_NAME.fish"
        if safe_remove_file "$fish_sys_completion" true; then
            print_success "Removed system fish completion"
            removed=true
        fi
    fi

    if [[ "$UNINSTALL_MODE" != "system" ]]; then
        # User bash completion
        if safe_remove_file "$USER_COMPLETION_DIR/$BINARY_NAME" false; then
            print_success "Removed user bash completion"
            removed=true
        fi

        # User zsh completion
        local zsh_user_completion="$HOME/.local/share/zsh/site-functions/_$BINARY_NAME"
        if safe_remove_file "$zsh_user_completion" false; then
            print_success "Removed user zsh completion"
            removed=true
        fi

        # User fish completion
        local fish_user_completion="$HOME/.config/fish/completions/$BINARY_NAME.fish"
        if safe_remove_file "$fish_user_completion" false; then
            print_success "Removed user fish completion"
            removed=true
        fi
    fi

    if [[ "$removed" != true ]]; then
        print_status "No completion files found to remove"
    fi
}

# Function to remove configuration
remove_configuration() {
    if [[ "$REMOVE_CONFIG" != true ]]; then
        print_status "Skipping configuration removal (use --remove-config to remove)"
        return
    fi

    local removed=false

    if [[ "$UNINSTALL_MODE" != "user" ]]; then
        if [[ -d "$SYSTEM_CONFIG_DIR" ]]; then
            if confirm_action "Remove system configuration directory: $SYSTEM_CONFIG_DIR"; then
                safe_remove_dir "$SYSTEM_CONFIG_DIR" true true
                print_success "Removed system configuration"
                removed=true
            fi
        fi
    fi

    if [[ "$UNINSTALL_MODE" != "system" ]]; then
        if [[ -d "$USER_CONFIG_DIR" ]]; then
            if confirm_action "Remove user configuration directory: $USER_CONFIG_DIR"; then
                safe_remove_dir "$USER_CONFIG_DIR" false true
                print_success "Removed user configuration"
                removed=true
            fi
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

    if [[ "$UNINSTALL_MODE" != "user" ]]; then
        if [[ -d "$SYSTEM_DATA_DIR" ]]; then
            if confirm_action "Remove system data directory (including downloads): $SYSTEM_DATA_DIR"; then
                safe_remove_dir "$SYSTEM_DATA_DIR" true true
                print_success "Removed system data directory"
                removed=true
            fi
        fi

        if [[ -d "$SYSTEM_LOG_DIR" ]]; then
            if confirm_action "Remove system log directory: $SYSTEM_LOG_DIR"; then
                safe_remove_dir "$SYSTEM_LOG_DIR" true true
                print_success "Removed system log directory"
                removed=true
            fi
        fi
    fi

    if [[ "$UNINSTALL_MODE" != "system" ]]; then
        if [[ -d "$USER_DATA_DIR" ]]; then
            if confirm_action "Remove user data directory: $USER_DATA_DIR"; then
                safe_remove_dir "$USER_DATA_DIR" false true
                print_success "Removed user data directory"
                removed=true
            fi
        fi
    fi

    # Security directories (checked regardless of mode)
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

    if [[ "$UNINSTALL_MODE" == "user" ]]; then
        print_status "Skipping system user removal in user mode"
        return
    fi

    if id "localgo" &>/dev/null; then
        if confirm_action "Remove system user 'localgo' and group"; then
            print_action "Remove user: localgo"
            if [[ "$DRY_RUN" != true ]]; then
                sudo userdel localgo 2>/dev/null || true
                sudo groupdel localgo 2>/dev/null || true
            fi
            print_success "Removed system user and group"
        fi
    else
        print_status "System user 'localgo' not found"
    fi
}

# Show a human-readable removal plan before doing anything
show_removal_plan() {
    local detection
    detection=$(detect_installation)

    local found_system_binary found_user_binary found_system_service found_user_service
    found_system_binary=$(echo "$detection"  | grep -o 'system_binary:[^[:space:]]*'  | cut -d: -f2)
    found_user_binary=$(echo "$detection"    | grep -o 'user_binary:[^[:space:]]*'    | cut -d: -f2)
    found_system_service=$(echo "$detection" | grep -o 'system_service:[^[:space:]]*' | cut -d: -f2)
    found_user_service=$(echo "$detection"   | grep -o 'user_service:[^[:space:]]*'   | cut -d: -f2)

    local user_svc
    user_svc=$(user_service_file)

    echo
    print_status "LocalGo Uninstall Plan  (mode: $UNINSTALL_MODE)"
    echo "==========================================="
    echo

    echo "Components to remove:"

    if [[ "$UNINSTALL_MODE" != "user" ]]; then
        if [[ "$found_system_binary" == true ]]; then
            echo "  + System binary:   $SYSTEM_BIN_DIR/$BINARY_NAME"
        fi
        if [[ "$found_system_service" == true ]]; then
            echo "  + System service:  $SYSTEM_SERVICE_FILE"
        fi
    fi

    if [[ "$UNINSTALL_MODE" != "system" ]]; then
        if [[ "$found_user_binary" == true ]]; then
            echo "  + User binary:     $USER_BIN_DIR/$BINARY_NAME"
        fi
        if [[ "$found_user_service" == true ]]; then
            echo "  + User service:    $user_svc"
        fi
    fi

    echo "  + Shell completion files"

    if [[ "$REMOVE_CONFIG" == true ]]; then
        echo "  + Configuration directories"
    else
        echo "  - Configuration directories  (preserved; use --remove-config)"
    fi

    if [[ "$REMOVE_DATA" == true ]]; then
        echo "  + Data directories and downloads"
        echo "  + Security certificates"
    else
        echo "  - Data directories and downloads  (preserved; use --remove-data)"
        echo "  - Security certificates           (preserved; use --remove-data)"
    fi

    if [[ "$UNINSTALL_MODE" != "user" ]]; then
        if [[ "$REMOVE_USER" == true ]]; then
            echo "  + System user 'localgo'"
        else
            echo "  - System user 'localgo'  (preserved; use --remove-user)"
        fi
    fi

    echo

    if [[ "$REMOVE_DATA" == true ]]; then
        print_warning "Data removal will delete all downloaded files!"
    fi
    if [[ "$REMOVE_CONFIG" == true ]]; then
        print_warning "Configuration removal will delete all settings!"
    fi

    echo
}

# Verify that removal succeeded
verify_removal() {
    print_status "Verifying removal..."

    local issues_found=false
    local user_svc
    user_svc=$(user_service_file)

    if [[ "$UNINSTALL_MODE" != "user" ]]; then
        if [[ -f "$SYSTEM_BIN_DIR/$BINARY_NAME" ]]; then
            print_warning "System binary still exists: $SYSTEM_BIN_DIR/$BINARY_NAME"
            issues_found=true
        fi
        if [[ -f "$SYSTEM_SERVICE_FILE" ]]; then
            print_warning "System service file still exists: $SYSTEM_SERVICE_FILE"
            issues_found=true
        fi
        if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            print_warning "System service is still running"
            issues_found=true
        fi
    fi

    if [[ "$UNINSTALL_MODE" != "system" ]]; then
        if [[ -f "$USER_BIN_DIR/$BINARY_NAME" ]]; then
            print_warning "User binary still exists: $USER_BIN_DIR/$BINARY_NAME"
            issues_found=true
        fi
        if [[ -f "$user_svc" ]]; then
            print_warning "User service file still exists: $user_svc"
            issues_found=true
        fi
        if systemctl --user is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            print_warning "User service is still running"
            issues_found=true
        fi
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
            --mode)
                UNINSTALL_MODE="$2"
                shift 2
                ;;
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

    # Validate mode early
    if [[ "$UNINSTALL_MODE" != "all" && "$UNINSTALL_MODE" != "user" && "$UNINSTALL_MODE" != "system" ]]; then
        print_error "Invalid mode: '$UNINSTALL_MODE'. Must be 'all', 'user', or 'system'."
        exit 1
    fi

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

    print_status "Starting LocalGo removal..."
    echo

    # Remove service first (stops any running processes)
    remove_service

    # Remove binaries
    remove_binaries

    # Remove shell completions
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
