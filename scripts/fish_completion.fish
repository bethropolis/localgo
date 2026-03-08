# Fish completion script for localgo

set -l commands serve discover scan send share devices info help version

# Disable file completion by default
complete -c localgo -f

# Main commands
complete -c localgo -n "not __fish_seen_subcommand_from $commands" -a serve -d "Start the LocalGo server"
complete -c localgo -n "not __fish_seen_subcommand_from $commands" -a discover -d "Discover devices on network"
complete -c localgo -n "not __fish_seen_subcommand_from $commands" -a scan -d "Scan network for devices"
complete -c localgo -n "not __fish_seen_subcommand_from $commands" -a send -d "Send a file"
complete -c localgo -n "not __fish_seen_subcommand_from $commands" -a share -d "Share files via web"
complete -c localgo -n "not __fish_seen_subcommand_from $commands" -a devices -d "Show recently discovered devices"
complete -c localgo -n "not __fish_seen_subcommand_from $commands" -a info -d "Show device info"
complete -c localgo -n "not __fish_seen_subcommand_from $commands" -a help -d "Show help"
complete -c localgo -n "not __fish_seen_subcommand_from $commands" -a version -d "Show version"

# Global flags
complete -c localgo -s h -l help -d "Show help"
complete -c localgo -s v -l version -d "Show version"

# Serve command flags
complete -c localgo -n "__fish_seen_subcommand_from serve" -l port -d "Port to listen on" -x
complete -c localgo -n "__fish_seen_subcommand_from serve" -l http -d "Use HTTP instead of HTTPS"
complete -c localgo -n "__fish_seen_subcommand_from serve" -l pin -d "Require PIN" -x
complete -c localgo -n "__fish_seen_subcommand_from serve" -l alias -d "Device alias" -x
complete -c localgo -n "__fish_seen_subcommand_from serve" -l dir -d "Download directory" -r
complete -c localgo -n "__fish_seen_subcommand_from serve" -l quiet -d "Quiet mode"
complete -c localgo -n "__fish_seen_subcommand_from serve" -l verbose -d "Verbose mode"
complete -c localgo -n "__fish_seen_subcommand_from serve" -l interval -d "Discovery announcement interval in seconds" -x
complete -c localgo -n "__fish_seen_subcommand_from serve" -l auto-accept -d "Auto-accept incoming files"
complete -c localgo -n "__fish_seen_subcommand_from serve" -l no-clipboard -d "Save incoming text as a file instead of clipboard"
complete -c localgo -n "__fish_seen_subcommand_from serve" -l history -d "Path to transfer history JSONL file" -r
complete -c localgo -n "__fish_seen_subcommand_from serve" -l exec -d "Shell command to execute after each received file" -x

# Discover command flags
complete -c localgo -n "__fish_seen_subcommand_from discover" -l timeout -d "Timeout in seconds" -x
complete -c localgo -n "__fish_seen_subcommand_from discover" -l json -d "Output JSON"
complete -c localgo -n "__fish_seen_subcommand_from discover" -l quiet -d "Quiet mode"

# Scan command flags
complete -c localgo -n "__fish_seen_subcommand_from scan" -l timeout -d "Timeout in seconds" -x
complete -c localgo -n "__fish_seen_subcommand_from scan" -l port -d "Port to scan" -x
complete -c localgo -n "__fish_seen_subcommand_from scan" -l json -d "Output JSON"
complete -c localgo -n "__fish_seen_subcommand_from scan" -l quiet -d "Quiet mode"

# Send command flags
complete -c localgo -n "__fish_seen_subcommand_from send" -l file -d "File to send" -r
complete -c localgo -n "__fish_seen_subcommand_from send" -l to -d "Target device alias" -x
complete -c localgo -n "__fish_seen_subcommand_from send" -l port -d "Target port" -x
complete -c localgo -n "__fish_seen_subcommand_from send" -l timeout -d "Timeout in seconds" -x
complete -c localgo -n "__fish_seen_subcommand_from send" -l alias -d "Sender alias" -x

# Share command flags
complete -c localgo -n "__fish_seen_subcommand_from share" -l file -d "File to share" -r
complete -c localgo -n "__fish_seen_subcommand_from share" -l port -d "Port to listen on" -x
complete -c localgo -n "__fish_seen_subcommand_from share" -l http -d "Use HTTP instead of HTTPS"
complete -c localgo -n "__fish_seen_subcommand_from share" -l pin -d "Require PIN" -x
complete -c localgo -n "__fish_seen_subcommand_from share" -l alias -d "Device alias" -x
complete -c localgo -n "__fish_seen_subcommand_from share" -l auto-accept -d "Auto-accept incoming files"
complete -c localgo -n "__fish_seen_subcommand_from share" -l no-clipboard -d "Save incoming text as a file instead of clipboard"
complete -c localgo -n "__fish_seen_subcommand_from share" -l history -d "Path to transfer history JSONL file" -r
complete -c localgo -n "__fish_seen_subcommand_from share" -l exec -d "Shell command to execute after each received file" -x
complete -c localgo -n "__fish_seen_subcommand_from share" -l quiet -d "Quiet mode"

# Devices command flags
complete -c localgo -n "__fish_seen_subcommand_from devices" -l json -d "Output JSON"

# Info command flags
complete -c localgo -n "__fish_seen_subcommand_from info" -l json -d "Output JSON"
