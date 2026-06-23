package help

// GetCommandHelp returns help information for built-in commands
func GetCommandHelp(commandName string) *CommandHelp {
	commands := map[string]*CommandHelp{
		"serve": {
			Name:        "serve",
			Description: "Start the LocalGo server to receive files",
			Usage:       "localgo serve [OPTIONS]",
			Examples: []string{
				"localgo serve",
				"localgo serve --port 8080 --http",
				"localgo serve --pin 123456 --alias MyDevice",
				"localgo serve --dir /tmp/downloads --verbose",
				"localgo serve --auto-accept --quiet",
				"localgo serve --no-clipboard",
				"localgo serve --exec 'notify-send \"Got: %f\"'",
			},
			Flags: []FlagHelp{
				{Name: "--port", Type: "int", Default: "from config", Description: "Port to run the server on"},
				{Name: "--http", Type: "bool", Default: "false", Description: "Use HTTP instead of HTTPS"},
				{Name: "--pin", Type: "string", Default: "", Description: "PIN for authentication"},
				{Name: "--alias", Type: "string", Default: "from config", Description: "Device alias"},
				{Name: "--dir", Type: "string", Default: "from config", Description: "Download directory"},
				{Name: "--interval", Type: "int", Default: "30", Description: "Discovery announcement interval in seconds"},
				{Name: "--auto-accept", Type: "bool", Default: "false", Description: "Auto-accept incoming files without prompting"},
				{Name: "--no-clipboard", Type: "bool", Default: "false", Description: "Save incoming text as a file instead of copying to clipboard"},
				{Name: "--open", Type: "bool", Default: "false", Description: "Open download directory after transfer completes"},
				{Name: "--quiet", Type: "bool", Default: "false", Description: "Quiet mode - minimal output"},
				{Name: "--verbose", Type: "bool", Default: "false", Description: "Verbose mode - detailed output"},
				{Name: "--history", Type: "string", Default: "~/.local/share/localgo/history.jsonl", Description: "Path to transfer history JSONL file"},
				{Name: "--exec", Type: "string", Default: "", Description: "Shell command to execute after each received file (use %f, %n, %s, %a, %i)"},
				{Name: "--iface", Type: "string", Default: "", Description: "Multicast network interface name"},
			},
		},
		"share": {
			Name:        "share",
			Description: "Share files so other devices can download them",
			Usage:       "localgo share --file FILE [OPTIONS]",
			Examples: []string{
				"localgo share --file document.pdf",
				"localgo share --file image.jpg --file text.txt",
				"localgo share --file data.zip --pin 1234",
				"localgo share --file data.zip --auto-accept",
				"localgo share --file report.pdf --no-clipboard",
				"localgo share --file doc.pdf --exec 'curl -F \"file=@%f\" https://example.com/upload'",
			},
			Flags: []FlagHelp{
				{Name: "--file", Type: "string", Default: "", Description: "File or directory to share (required, can be specified multiple times)"},
				{Name: "--port", Type: "int", Default: "from config", Description: "Port to run the server on"},
				{Name: "--http", Type: "bool", Default: "false", Description: "Use HTTP instead of HTTPS"},
				{Name: "--pin", Type: "string", Default: "", Description: "PIN for authentication"},
				{Name: "--alias", Type: "string", Default: "from config", Description: "Device alias"},
				{Name: "--auto-accept", Type: "bool", Default: "false", Description: "Auto-accept incoming files without prompting"},
				{Name: "--no-clipboard", Type: "bool", Default: "false", Description: "Save incoming text as a file instead of copying to clipboard"},
				{Name: "--zip", Type: "bool", Default: "false", Description: "Zip directories before sharing"},
				{Name: "--concurrency", Type: "int", Default: "0", Description: "Max parallel uploads (0 = use default)"},
				{Name: "--history", Type: "string", Default: "", Description: "Path to transfer history JSONL file"},
				{Name: "--exec", Type: "string", Default: "", Description: "Shell command to execute after each received file"},
				{Name: "--quiet", Type: "bool", Default: "false", Description: "Quiet mode - minimal output"},
				{Name: "--iface", Type: "string", Default: "", Description: "Multicast network interface name"},
			},
		},
		"discover": {
			Name:        "discover",
			Description: "Discover LocalGo devices on the network using multicast",
			Usage:       "localgo discover [OPTIONS]",
			Examples: []string{
				"localgo discover",
				"localgo discover --timeout 10",
				"localgo discover --json",
				"localgo discover --quiet",
			},
			Flags: []FlagHelp{
				{Name: "--timeout", Type: "int", Default: "10", Description: "Discovery timeout in seconds"},
				{Name: "--json", Type: "bool", Default: "false", Description: "Output in JSON format"},
				{Name: "--quiet", Type: "bool", Default: "false", Description: "Quiet mode - only show results"},
			},
		},
		"scan": {
			Name:        "scan",
			Description: "Scan the network for LocalGo devices using HTTP",
			Usage:       "localgo scan [OPTIONS]",
			Examples: []string{
				"localgo scan",
				"localgo scan --port 8080 --timeout 30",
				"localgo scan --json",
				"localgo scan --quiet",
				"localgo scan --range 192.168.1.0/24",
			},
			Flags: []FlagHelp{
				{Name: "--range", Type: "string", Default: "", Description: "CIDR range to scan (e.g. 192.168.1.0/24)"},
				{Name: "--timeout", Type: "int", Default: "15", Description: "Scan timeout in seconds"},
				{Name: "--port", Type: "int", Default: "from config", Description: "Port to scan"},
				{Name: "--json", Type: "bool", Default: "false", Description: "Output in JSON format"},
				{Name: "--quiet", Type: "bool", Default: "false", Description: "Quiet mode - only show results"},
			},
		},
		"send": {
			Name:        "send",
			Description: "Send a file or clipboard text to another LocalGo device",
			Usage:       "localgo send [OPTIONS]",
			Examples: []string{
				"localgo send --file document.pdf --to MyPhone",
				"localgo send --ip 192.168.1.42 --file document.pdf",
				"localgo send --ip 192.168.1.42:53317 --file document.pdf",
				"localgo send --clipboard --to MyPhone",
				"localgo send -c --to MyPhone",
				"localgo send (starts interactive clipboard or file picker if empty)",
			},
			Flags: []FlagHelp{
				{Name: "--file", Type: "string", Default: "", Description: "File or directory to send (optional, can be specified multiple times)"},
				{Name: "--ip", Type: "string", Default: "", Description: "Target device IP (with optional :port, skips discovery)"},
				{Name: "--to", Type: "string", Default: "", Description: "Target device alias (omit to pick interactively)"},
				{Name: "--clipboard, -c", Type: "bool", Default: "false", Description: "Send current system clipboard text directly"},
				{Name: "--port", Type: "int", Default: "auto-detect", Description: "Target device port"},
				{Name: "--timeout", Type: "int", Default: "30", Description: "Send timeout in seconds"},
				{Name: "--alias", Type: "string", Default: "from config", Description: "Sender alias"},
				{Name: "--concurrency", Type: "int", Default: "0", Description: "Max parallel uploads (0 = use default)"},
				{Name: "--iface", Type: "string", Default: "", Description: "Multicast network interface name"},
			},
		},
		"history": {
			Name:        "history",
			Description: "Show file transfer history log",
			Usage:       "localgo history [OPTIONS]",
			Examples: []string{
				"localgo history",
				"localgo history --limit 20",
				"localgo history --clear",
			},
			Flags: []FlagHelp{
				{Name: "--limit", Type: "int", Default: "10", Description: "Maximum number of entries to display"},
				{Name: "--clear", Type: "bool", Default: "false", Description: "Clear all transfer history logs"},
			},
		},
		"devices": {
			Name:        "devices",
			Description: "List recently discovered devices on the network",
			Usage:       "localgo devices [OPTIONS]",
			Examples: []string{
				"localgo devices",
				"localgo devices --probe",
				"localgo devices --json",
			},
			Flags: []FlagHelp{
				{Name: "--probe, -p", Type: "bool", Default: "false", Description: "Probe cached devices to verify if they are currently online"},
				{Name: "--json", Type: "bool", Default: "false", Description: "Output in JSON format"},
			},
		},
		"info": {
			Name:        "info",
			Description: "Show device information and configuration",
			Usage:       "localgo info [OPTIONS]",
			Examples: []string{
				"localgo info",
				"localgo info --json",
			},
			Flags: []FlagHelp{
				{Name: "--json", Type: "bool", Default: "false", Description: "Output in JSON format"},
			},
		},
		"completion": {
			Name:        "completion",
			Description: "Generate shell completion scripts",
			Usage:       "localgo completion [bash|zsh|fish|powershell]",
			Examples: []string{
				"localgo completion bash > /etc/bash_completion.d/localgo",
				"localgo completion zsh > /usr/local/share/zsh/site-functions/_localgo",
				"localgo completion fish > ~/.config/fish/completions/localgo.fish",
			},
			Flags: []FlagHelp{},
		},
	}

	return commands[commandName]
}
