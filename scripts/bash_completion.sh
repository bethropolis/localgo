#!/bin/bash

# Bash completion script for localgo-cli
# To use this script, source it in your ~/.bashrc or copy it to /etc/bash_completion.d/

_localgo_cli_completions() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Main commands
    local commands="serve discover scan send info help version"

    # Global flags
    local global_flags="-h --help -v --version"

    # Serve command flags
    local serve_flags="--port --http --pin --alias --dir --quiet --verbose"

    # Discover command flags
    local discover_flags="--timeout --json --quiet"

    # Scan command flags
    local scan_flags="--timeout --port --json --quiet"

    # Send command flags
    local send_flags="--file --to --port --timeout --alias"

    # Info command flags
    local info_flags="--json"

    # Get the command (first argument after localgo-cli)
    local command=""
    if [[ ${#COMP_WORDS[@]} -gt 1 ]]; then
        command="${COMP_WORDS[1]}"
    fi

    # If we're completing the first argument (command)
    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=($(compgen -W "${commands} ${global_flags}" -- ${cur}))
        return 0
    fi

    # Handle flag values
    case "${prev}" in
        --file)
            # Complete file paths
            COMPREPLY=($(compgen -f -- ${cur}))
            return 0
            ;;
        --dir)
            # Complete directory paths
            COMPREPLY=($(compgen -d -- ${cur}))
            return 0
            ;;
        --port)
            # Suggest common ports
            COMPREPLY=($(compgen -W "53317 8080 8000 3000 5000" -- ${cur}))
            return 0
            ;;
        --timeout)
            # Suggest common timeout values (in seconds)
            COMPREPLY=($(compgen -W "5 10 15 30 60 120" -- ${cur}))
            return 0
            ;;
        --pin)
            # Don't complete PIN values for security
            COMPREPLY=()
            return 0
            ;;
        --alias)
            # Suggest some common aliases
            COMPREPLY=($(compgen -W "MyDevice Laptop Desktop Phone Tablet Server" -- ${cur}))
            return 0
            ;;
        --to)
            # Try to get device list from recent discovery (if available)
            # For now, suggest common device names
            COMPREPLY=($(compgen -W "MyPhone MyLaptop MyDesktop MyTablet" -- ${cur}))
            return 0
            ;;
    esac

    # Complete flags based on the command
    case "${command}" in
        serve)
            if [[ ${cur} == -* ]]; then
                COMPREPLY=($(compgen -W "${serve_flags}" -- ${cur}))
            fi
            ;;
        discover)
            if [[ ${cur} == -* ]]; then
                COMPREPLY=($(compgen -W "${discover_flags}" -- ${cur}))
            fi
            ;;
        scan)
            if [[ ${cur} == -* ]]; then
                COMPREPLY=($(compgen -W "${scan_flags}" -- ${cur}))
            fi
            ;;
        send)
            if [[ ${cur} == -* ]]; then
                COMPREPLY=($(compgen -W "${send_flags}" -- ${cur}))
            fi
            ;;
        info)
            if [[ ${cur} == -* ]]; then
                COMPREPLY=($(compgen -W "${info_flags}" -- ${cur}))
            fi
            ;;
        help)
            # Complete with available commands for help
            COMPREPLY=($(compgen -W "${commands}" -- ${cur}))
            ;;
        version)
            # No flags for version command
            COMPREPLY=()
            ;;
        *)
            # Unknown command, suggest main commands
            COMPREPLY=($(compgen -W "${commands}" -- ${cur}))
            ;;
    esac

    return 0
}

# Register the completion function
complete -F _localgo_cli_completions localgo-cli

# Also register for common aliases
complete -F _localgo_cli_completions localgo
complete -F _localgo_cli_completions lg

# Function to install completion
install_localgo_completion() {
    local completion_dir="/etc/bash_completion.d"
    local user_completion_dir="$HOME/.local/share/bash-completion/completions"

    echo "Installing LocalGo CLI bash completion..."

    # Try system-wide installation first
    if [[ -w "$completion_dir" ]]; then
        cp "$(dirname "$0")/bash_completion.sh" "$completion_dir/localgo-cli"
        echo "Installed system-wide completion to $completion_dir/localgo-cli"
    elif [[ ! -d "$user_completion_dir" ]]; then
        mkdir -p "$user_completion_dir"
        cp "$(dirname "$0")/bash_completion.sh" "$user_completion_dir/localgo-cli"
        echo "Installed user completion to $user_completion_dir/localgo-cli"
    else
        cp "$(dirname "$0")/bash_completion.sh" "$user_completion_dir/localgo-cli"
        echo "Installed user completion to $user_completion_dir/localgo-cli"
    fi

    echo "Please restart your shell or run 'source ~/.bashrc' to enable completion."
}

# Function to show completion help
show_completion_help() {
    cat << 'EOF'
LocalGo CLI Bash Completion

This script provides intelligent tab completion for the localgo-cli command.

Features:
- Command completion (serve, discover, scan, send, info, help, version)
- Flag completion for each command
- File path completion for --file flags
- Directory path completion for --dir flags
- Smart suggestions for common values (ports, timeouts, etc.)

Installation:
1. Source this script in your ~/.bashrc:
   echo "source /path/to/bash_completion.sh" >> ~/.bashrc

2. Or install system-wide:
   sudo cp bash_completion.sh /etc/bash_completion.d/localgo-cli

3. Or use the installer function:
   source bash_completion.sh && install_localgo_completion

Usage:
After installation, you can use tab completion with localgo-cli:

  localgo-cli <TAB>                    # Shows available commands
  localgo-cli serve --<TAB>            # Shows serve command flags
  localgo-cli send --file <TAB>        # Completes file paths
  localgo-cli send --to <TAB>          # Suggests device names

Examples:
  localgo-cli se<TAB>                  # Completes to "serve"
  localgo-cli serve --p<TAB>           # Completes to "--port"
  localgo-cli send --file ~/Doc<TAB>   # Completes file path
EOF
}

# If script is run directly, show help
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    case "${1:-}" in
        install)
            install_localgo_completion
            ;;
        help|--help|-h)
            show_completion_help
            ;;
        *)
            echo "LocalGo CLI Bash Completion Script"
            echo ""
            echo "Usage:"
            echo "  source $0                 # Load completion into current shell"
            echo "  $0 install               # Install completion system-wide"
            echo "  $0 help                  # Show detailed help"
            echo ""
            echo "To enable completion, source this script in your ~/.bashrc or install it."
            ;;
    esac
fi
