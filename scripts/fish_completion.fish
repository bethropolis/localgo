# Fish completion script for localgo-cli

set -l commands serve discover scan send info help version

# Disable file completion by default
complete -c localgo-cli -f

# Main commands
complete -c localgo-cli -n "not __fish_seen_subcommand_from $commands" -a serve -d "Start the LocalGo server"
complete -c localgo-cli -n "not __fish_seen_subcommand_from $commands" -a discover -d "Discover devices on network"
complete -c localgo-cli -n "not __fish_seen_subcommand_from $commands" -a scan -d "Scan network for devices"
complete -c localgo-cli -n "not __fish_seen_subcommand_from $commands" -a send -d "Send a file"
complete -c localgo-cli -n "not __fish_seen_subcommand_from $commands" -a info -d "Show device info"
complete -c localgo-cli -n "not __fish_seen_subcommand_from $commands" -a help -d "Show help"
complete -c localgo-cli -n "not __fish_seen_subcommand_from $commands" -a version -d "Show version"

# Global flags
complete -c localgo-cli -s h -l help -d "Show help"
complete -c localgo-cli -s v -l version -d "Show version"

# Serve command flags
complete -c localgo-cli -n "__fish_seen_subcommand_from serve" -l port -d "Port to listen on" -x
complete -c localgo-cli -n "__fish_seen_subcommand_from serve" -l http -d "Use HTTP instead of HTTPS"
complete -c localgo-cli -n "__fish_seen_subcommand_from serve" -l pin -d "Require PIN" -x
complete -c localgo-cli -n "__fish_seen_subcommand_from serve" -l alias -d "Device alias" -x
complete -c localgo-cli -n "__fish_seen_subcommand_from serve" -l dir -d "Download directory" -r
complete -c localgo-cli -n "__fish_seen_subcommand_from serve" -l quiet -d "Quiet mode"
complete -c localgo-cli -n "__fish_seen_subcommand_from serve" -l verbose -d "Verbose mode"

# Discover command flags
complete -c localgo-cli -n "__fish_seen_subcommand_from discover" -l timeout -d "Timeout in seconds" -x
complete -c localgo-cli -n "__fish_seen_subcommand_from discover" -l json -d "Output JSON"
complete -c localgo-cli -n "__fish_seen_subcommand_from discover" -l quiet -d "Quiet mode"

# Scan command flags
complete -c localgo-cli -n "__fish_seen_subcommand_from scan" -l timeout -d "Timeout in seconds" -x
complete -c localgo-cli -n "__fish_seen_subcommand_from scan" -l port -d "Port to scan" -x
complete -c localgo-cli -n "__fish_seen_subcommand_from scan" -l json -d "Output JSON"
complete -c localgo-cli -n "__fish_seen_subcommand_from scan" -l quiet -d "Quiet mode"

# Send command flags
complete -c localgo-cli -n "__fish_seen_subcommand_from send" -l file -d "File to send" -r
complete -c localgo-cli -n "__fish_seen_subcommand_from send" -l to -d "Target device alias" -x
complete -c localgo-cli -n "__fish_seen_subcommand_from send" -l port -d "Target port" -x
complete -c localgo-cli -n "__fish_seen_subcommand_from send" -l timeout -d "Timeout in seconds" -x
complete -c localgo-cli -n "__fish_seen_subcommand_from send" -l alias -d "Sender alias" -x

# Info command flags
complete -c localgo-cli -n "__fish_seen_subcommand_from info" -l json -d "Output JSON"
