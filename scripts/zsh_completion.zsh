#compdef localgo

_localgo() {
    local line state

    _arguments -C \
        '-h[Show help]' \
        '--help[Show help]' \
        '-v[Show version]' \
        '--version[Show version]' \
        '--verbose[Verbose output]' \
        '--json[JSON output]' \
        '1: :->cmds' \
        '*:: :->args'

    case "$state" in
        cmds)
            local -a commands
            commands=(
                "serve:Start the LocalGo server to receive files"
                "discover:Discover LocalGo devices on the network"
                "scan:Scan the network for LocalGo devices using HTTP"
                "send:Send a file to another LocalGo device"
                "share:Share files so others can download them"
                "devices:Show all recently discovered devices"
                "info:Show device information and configuration"
                "help:Show help"
                "version:Show version"
            )
            _describe -t commands 'localgo commands' commands
            ;;
        args)
            case $line[1] in
                serve)
                    _arguments \
                        '--port=[Port to run the server on]:port:' \
                        '--http[Use HTTP instead of HTTPS]' \
                        '--pin=[PIN for authentication]:pin:' \
                        '--alias=[Device alias]:alias:' \
                        '--dir=[Download directory]:directory:_files -/' \
                        '--quiet[Quiet mode - minimal output]' \
                        '--verbose[Verbose mode - detailed output]' \
                        '--interval=[Discovery announcement interval in seconds]:interval:' \
                        '--auto-accept[Auto-accept incoming files without prompting]' \
                        '--no-clipboard[Save incoming text as a file instead of copying to clipboard]' \
                        '--history=[Path to transfer history JSONL file]:file:_files' \
                        '--exec=[Shell command to execute after each received file]:command:'
                    ;;
                discover)
                    _arguments \
                        '--timeout=[Discovery timeout in seconds]:timeout:' \
                        '--json[Output in JSON format]' \
                        '--quiet[Quiet mode - only show results]'
                    ;;
                scan)
                    _arguments \
                        '--timeout=[Scan timeout in seconds]:timeout:' \
                        '--port=[Port to scan]:port:' \
                        '--json[Output in JSON format]' \
                        '--quiet[Quiet mode - only show results]'
                    ;;
                send)
                    _arguments \
                        '*--file=[File or directory to send]:file:_files' \
                        '--to=[Target device alias]:alias:' \
                        '--port=[Target device port]:port:' \
                        '--timeout=[Send timeout in seconds]:timeout:' \
                        '--alias=[Sender alias]:alias:'
                    ;;
                share)
                    _arguments \
                        '*--file=[File or directory to share]:file:_files' \
                        '--port=[Port to run the server on]:port:' \
                        '--http[Use HTTP instead of HTTPS]' \
                        '--pin=[PIN for authentication]:pin:' \
                        '--alias=[Device alias]:alias:' \
                        '--auto-accept[Auto-accept incoming files without prompting]' \
                        '--no-clipboard[Save incoming text as a file instead of copying to clipboard]' \
                        '--history=[Path to transfer history JSONL file]:file:_files' \
                        '--exec=[Shell command to execute after each received file]:command:' \
                        '--quiet[Quiet mode - minimal output]'
                    ;;
                devices)
                    _arguments \
                        '--json[Output in JSON format]'
                    ;;
                info)
                    _arguments \
                        '--json[Output in JSON format]'
                    ;;
            esac
            ;;
    esac
}

_localgo "$@"
