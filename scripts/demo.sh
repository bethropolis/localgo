#!/bin/bash

# LocalGo CLI Demo Script
# This script demonstrates the improved terminal/headless experience

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Demo configuration
BINARY_PATH="${1:-/tmp/localgo-cli}"
DEMO_FILE="/tmp/demo-file.txt"
DEMO_PORT1=8080
DEMO_PORT2=8081

# Function to print section headers
print_section() {
    echo
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE} $1${NC}"
    echo -e "${BLUE}================================================${NC}"
    echo
}

# Function to print demo steps
print_step() {
    echo -e "${GREEN}➤ $1${NC}"
    echo
}

# Function to run command with description
run_demo() {
    local description="$1"
    shift
    echo -e "${YELLOW}Demo:${NC} $description"
    echo -e "${CYAN}Command:${NC} $*"
    echo
    "$@"
    echo
    echo -e "${PURPLE}Press Enter to continue...${NC}"
    read -r
}

# Function to cleanup
cleanup() {
    echo -e "${YELLOW}Cleaning up demo...${NC}"
    pkill -f "localgo-cli serve" 2>/dev/null || true
    rm -f "$DEMO_FILE" 2>/dev/null || true
    echo -e "${GREEN}Demo cleanup complete${NC}"
}

# Trap cleanup on exit
trap cleanup EXIT

# Check if binary exists
if [[ ! -x "$BINARY_PATH" ]]; then
    echo -e "${RED}Error: LocalGo binary not found at $BINARY_PATH${NC}"
    echo "Please build it first with: make build"
    exit 1
fi

# Create demo file
echo "Hello from LocalGo CLI Demo! 🚀" > "$DEMO_FILE"
echo "This file demonstrates the improved file transfer experience." >> "$DEMO_FILE"
echo "Timestamp: $(date)" >> "$DEMO_FILE"

# Start demo
echo -e "${GREEN}LocalGo CLI Demo - Improved Terminal Experience${NC}"
echo -e "${GREEN}===============================================${NC}"
echo
echo "This demo showcases the enhanced CLI features:"
echo "• Comprehensive help system"
echo "• Better parameter handling"
echo "• JSON output for scripting"
echo "• Improved user experience"
echo "• Configuration options"
echo "• Embedded/headless operation"
echo
echo -e "${PURPLE}Press Enter to start the demo...${NC}"
read -r

# === HELP SYSTEM DEMO ===
print_section "1. Comprehensive Help System"

print_step "General help with all commands"
run_demo "Show main help menu" "$BINARY_PATH" help

print_step "Command-specific help"
run_demo "Help for send command" "$BINARY_PATH" help send

run_demo "Help for serve command" "$BINARY_PATH" help serve

# === VERSION AND INFO ===
print_section "2. Version and Device Information"

print_step "Version information"
run_demo "Show version with build details" "$BINARY_PATH" version

print_step "Device information (human-readable)"
run_demo "Show device configuration" "$BINARY_PATH" info

print_step "Device information (JSON for scripting)"
run_demo "Show device info in JSON format" "$BINARY_PATH" info --json

# === DISCOVERY FEATURES ===
print_section "3. Enhanced Discovery Options"

print_step "Quick device discovery"
run_demo "Discover devices with timeout" timeout 5 "$BINARY_PATH" discover --timeout 3 --quiet

print_step "Network scanning with JSON output"
run_demo "Scan network in JSON format" timeout 5 "$BINARY_PATH" scan --timeout 3 --json

# === CONFIGURATION DEMO ===
print_section "4. Configuration and Environment Variables"

print_step "Custom configuration via environment"
run_demo "Run with custom alias and port" \
    env LOCALSEND_ALIAS="Demo-Device" LOCALSEND_PORT=8080 "$BINARY_PATH" info

print_step "Custom download directory"
run_demo "Run with custom download directory" \
    env LOCALSEND_DOWNLOAD_DIR="/tmp/demo-downloads" "$BINARY_PATH" info --json

# === SERVER DEMO ===
print_section "5. Server Features and Options"

print_step "Start server with custom options"
echo -e "${YELLOW}Starting demo server in background...${NC}"
env LOCALSEND_ALIAS="Demo-Server" "$BINARY_PATH" serve \
    --port "$DEMO_PORT1" \
    --http \
    --dir "/tmp/demo-downloads" \
    --alias "Demo-Server" \
    --verbose &
SERVER_PID=$!

# Wait for server to start
sleep 3

run_demo "Test server is running" curl -s "http://localhost:$DEMO_PORT1/api/localsend/v2/info" | head -5

# Kill the server
kill $SERVER_PID 2>/dev/null || true
sleep 1

# === FILE TRANSFER DEMO ===
print_section "6. File Transfer with Enhanced UX"

print_step "Start receiving server"
echo -e "${YELLOW}Starting receiver server...${NC}"
env LOCALSEND_ALIAS="Receiver" "$BINARY_PATH" serve \
    --port "$DEMO_PORT1" \
    --http \
    --dir "/tmp/demo-downloads" \
    --quiet &
RECEIVER_PID=$!
sleep 2

print_step "Send file with progress and feedback"
run_demo "Send demo file to receiver" \
    env LOCALSEND_ALIAS="Sender" "$BINARY_PATH" send \
    --file "$DEMO_FILE" \
    --to "Receiver" \
    --port "$DEMO_PORT1" \
    --timeout 10

# Kill receiver
kill $RECEIVER_PID 2>/dev/null || true
sleep 1

print_step "Verify file was received"
run_demo "Check received file" ls -la /tmp/demo-downloads/ || echo "No files received"
run_demo "Show file content" cat /tmp/demo-downloads/demo-file.txt 2>/dev/null || echo "File not found"

# === HEADLESS/EMBEDDED FEATURES ===
print_section "7. Headless/Embedded Operation"

print_step "Quiet mode for scripting"
run_demo "Discovery in quiet mode (tab-separated)" "$BINARY_PATH" discover --quiet --timeout 2

print_step "JSON output for automation"
run_demo "Device info for scripts" "$BINARY_PATH" info --json

print_step "Error handling demonstration"
run_demo "Attempt to send non-existent file" "$BINARY_PATH" send --file "/nonexistent" --to "TestDevice" 2>&1 || true

# === CONFIGURATION FILES ===
print_section "8. Configuration Management"

print_step "Sample configuration file"
echo -e "${YELLOW}Sample configuration (localgo.env):${NC}"
cat << 'EOF'
# LocalGo Configuration
LOCALSEND_ALIAS="Production Server"
LOCALSEND_PORT=53317
LOCALSEND_DOWNLOAD_DIR="/srv/localgo/downloads"
LOCALSEND_PIN="secure123"
LOCALSEND_DEVICE_TYPE="server"
EOF
echo

print_step "Using configuration file"
run_demo "Example with environment file" \
    bash -c 'export LOCALSEND_ALIAS="Configured Device"; export LOCALSEND_PORT=9999; '"$BINARY_PATH"' info'

# === SYSTEMD SERVICE ===
print_section "9. System Service Integration"

print_step "Systemd service file demonstration"
echo -e "${YELLOW}Sample systemd service configuration:${NC}"
cat << 'EOF'
[Unit]
Description=LocalGo File Sharing Service
After=network.target

[Service]
Type=simple
User=localgo
ExecStart=/usr/local/bin/localgo-cli serve
EnvironmentFile=/etc/localgo/localgo.env
Restart=always

[Install]
WantedBy=multi-user.target
EOF
echo

echo -e "${CYAN}Service commands:${NC}"
echo "sudo systemctl enable localgo"
echo "sudo systemctl start localgo"
echo "sudo systemctl status localgo"
echo "sudo journalctl -u localgo -f"
echo

print_step "Installation script demonstration"
echo -e "${YELLOW}Installation script usage:${NC}"
echo "./scripts/install.sh --mode system --service --create-user"
echo

# === BASH COMPLETION ===
print_section "10. Bash Completion Features"

print_step "Bash completion demonstration"
echo -e "${YELLOW}Available completion features:${NC}"
echo "• Command completion: localgo-cli <TAB>"
echo "• Flag completion: localgo-cli serve --<TAB>"
echo "• File path completion: localgo-cli send --file <TAB>"
echo "• Smart suggestions for common values"
echo
echo -e "${CYAN}To enable completion:${NC}"
echo "source scripts/bash_completion.sh"
echo "# or install system-wide with the installation script"
echo

# === ADVANCED FEATURES ===
print_section "11. Advanced Features for Power Users"

print_step "Multiple output formats"
run_demo "Device list in different formats" "$BINARY_PATH" scan --timeout 2 --quiet

print_step "Debugging and troubleshooting"
echo -e "${YELLOW}Debugging features:${NC}"
echo "• --verbose flag for detailed output"
echo "• --quiet flag for minimal output"
echo "• JSON output for parsing"
echo "• Comprehensive error messages"
echo

print_step "Integration examples"
echo -e "${YELLOW}Integration examples:${NC}"
cat << 'EOF'
# Shell script integration
DEVICES=$(localgo-cli scan --json --timeout 5)
echo "$DEVICES" | jq '.devices[].alias'

# Service monitoring
systemctl is-active localgo && echo "Service running"

# Automated file sending
find /uploads -name "*.pdf" | while read file; do
    localgo-cli send --file "$file" --to "PrintServer"
done

# Health check
localgo-cli info --json | jq -r '.alias + " on port " + (.port|tostring)'
EOF
echo

# === DEMO CONCLUSION ===
print_section "Demo Complete! 🎉"

echo -e "${GREEN}Summary of improvements:${NC}"
echo "✅ Comprehensive help system with examples"
echo "✅ Better command-line argument handling"
echo "✅ JSON output for scripting and automation"
echo "✅ Improved error messages and user feedback"
echo "✅ Environment variable configuration"
echo "✅ Systemd service integration"
echo "✅ Bash completion for better UX"
echo "✅ Installation script for easy deployment"
echo "✅ Quiet and verbose modes"
echo "✅ Enhanced file transfer experience"
echo
echo -e "${BLUE}Next steps:${NC}"
echo "• Try the installation script: ./scripts/install.sh"
echo "• Enable bash completion for better terminal experience"
echo "• Set up systemd service for production deployment"
echo "• Customize configuration via environment variables"
echo "• Use JSON output for integration with other tools"
echo
echo -e "${YELLOW}For more information:${NC}"
echo "localgo-cli help"
echo "localgo-cli help <command>"
echo
echo -e "${GREEN}Thank you for trying LocalGo! 🚀${NC}"
