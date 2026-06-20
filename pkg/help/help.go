package help

import (
	"fmt"

	"github.com/bethropolis/localgo/pkg/cli"
)

// CommandHelp represents help information for a command
type CommandHelp struct {
	Name        string
	Description string
	Usage       string
	Examples    []string
	Flags       []FlagHelp
}

// FlagHelp represents help information for a flag
type FlagHelp struct {
	Name        string
	Type        string
	Default     string
	Description string
}

// ShowMainUsage displays the main help screen with colored output
func ShowMainUsage() {
	header := cli.HeaderStyle.Render("LocalGo CLI")
	subheader := cli.InfoStyle.Render("LocalSend v2.1 Protocol Implementation")

	fmt.Printf("%s - %s\n\n", header, subheader)

	fmt.Printf("%s\n", cli.WarningStyle.Render("USAGE:"))
	fmt.Printf("    localgo <COMMAND> [OPTIONS]\n\n")

	fmt.Printf("%s\n", cli.WarningStyle.Render("COMMANDS:"))
	commands := []struct {
		name, desc string
	}{
		{"serve", "Start the LocalGo server to receive files"},
		{"share", "Share files so other devices can download them"},
		{"send", "Send a file or clipboard text to another device"},
		{"discover", "Discover devices using multicast"},
		{"scan", "Scan network for devices using HTTP"},
		{"devices", "List recently discovered devices"},
		{"history", "Show file transfer history log"},
		{"info", "Show device information"},
		{"completion", "Generate shell completion scripts"},
		{"help", "Show help information"},
		{"version", "Show version information"},
	}

	for _, cmd := range commands {
		fmt.Printf("    %-12s %s\n", cli.SuccessStyle.Render(cmd.name), cmd.desc)
	}

	fmt.Printf("\n%s\n", cli.WarningStyle.Render("OPTIONS:"))
	fmt.Printf("    %s  Show help\n", cli.InfoStyle.Render("-h, --help"))
	fmt.Printf("    %s  Show version\n", cli.InfoStyle.Render("-v, --version"))
	fmt.Printf("    %s      Enable debug logging\n", cli.InfoStyle.Render("--verbose"))
	fmt.Printf("    %s         Enable JSON log output\n", cli.InfoStyle.Render("--json"))
	fmt.Printf("    %s          Hide device identity during discovery/transfer\n", cli.InfoStyle.Render("--private, -p"))
	fmt.Printf("    %s        Config file path\n\n", cli.InfoStyle.Render("--config"))

	fmt.Printf("%s\n", cli.WarningStyle.Render("EXAMPLES:"))
	examples := []string{
		"localgo serve --port 8080 --http",
		"localgo discover --timeout 10",
		"localgo send --file document.pdf --to MyPhone",
		"localgo send --clipboard --to MyPhone",
		"localgo share --file document.pdf",
		"localgo history --limit 20",
		"localgo help send",
	}

	for _, ex := range examples {
		fmt.Printf("    %s\n", cli.SuccessStyle.Render(ex))
	}

	fmt.Printf("\n%s\n", cli.HeaderStyle.Render("For more information about a specific command, use:"))
	fmt.Printf("    localgo help <COMMAND>\n\n")

	fmt.Printf("%s\n", cli.WarningStyle.Render("ENVIRONMENT VARIABLES:"))
	envVars := []struct {
		name, desc string
	}{
		{"LOCALSEND_ALIAS", "Device alias"},
		{"LOCALSEND_PORT", "Default port"},
		{"LOCALSEND_DOWNLOAD_DIR", "Download directory"},
		{"LOCALSEND_PIN", "Security PIN"},
		{"LOCALSEND_FORCE_HTTP", "Use HTTP instead of HTTPS"},
		{"LOCALSEND_DEVICE_TYPE", "Device type (mobile/desktop/server/laptop/tablet/headless/web/other)"},
		{"LOCALSEND_DEVICE_MODEL", "Device model string"},
		{"LOCALSEND_AUTO_ACCEPT", "Auto-accept incoming files (true/1)"},
		{"LOCALSEND_NO_CLIPBOARD", "Save incoming text as file instead of clipboard (true/1)"},
		{"LOCALSEND_QUIET", "Quiet mode - minimal output (true/1)"},
		{"LOCALSEND_HISTORY", "Path to transfer history JSONL file"},
		{"LOCALSEND_EXEC", "Shell command to execute after each received file"},
		{"LOCALSEND_MULTICAST_GROUP", "Multicast group address"},
		{"LOCALSEND_SECURITY_DIR", "Security directory path"},
		{"LOCALSEND_LOG_LEVEL", "Log verbosity (debug/info/warn/error)"},
	}

	for _, env := range envVars {
		fmt.Printf("    %-28s %s\n", cli.InfoStyle.Render(env.name), env.desc)
	}
	fmt.Println()
}

// ShowCommandHelp displays help for a specific command
func ShowCommandHelp(help CommandHelp) {
	header := cli.HeaderStyle.Render("LocalGo CLI")

	fmt.Printf("%s - %s\n", header, help.Description)
	fmt.Printf("\n%s\n", cli.WarningStyle.Render("USAGE:"))
	fmt.Printf("    %s\n\n", help.Usage)

	if len(help.Examples) > 0 {
		fmt.Printf("%s\n", cli.WarningStyle.Render("EXAMPLES:"))
		for _, example := range help.Examples {
			fmt.Printf("    %s\n", cli.SuccessStyle.Render(example))
		}
		fmt.Println()
	}

	if len(help.Flags) > 0 {
		fmt.Printf("%s\n", cli.WarningStyle.Render("OPTIONS:"))
		for _, flag := range help.Flags {
			flagName := cli.InfoStyle.Render(flag.Name)
			defaultVal := ""
			if flag.Default != "" {
				defaultVal = fmt.Sprintf(" %s", cli.MutedStyle.Render(fmt.Sprintf("(default: %s)", flag.Default)))
			}
			fmt.Printf("    %-30s %s%s\n", flagName, flag.Description, defaultVal)
		}
		fmt.Println()
	}
}

// ShowVersion displays version information with colors
func ShowVersion(version, commit, date string) {
	fmt.Printf("%s %s\n",
		cli.HeaderStyle.Render("LocalGo CLI"),
		cli.SuccessStyle.Render(version))
	fmt.Printf("%s %s\n",
		cli.HighlightStyle.Render("Git Commit:"),
		commit)
	fmt.Printf("%s %s\n",
		cli.HighlightStyle.Render("Build Date:"),
		date)
	fmt.Printf("%s %s\n",
		cli.HighlightStyle.Render("Protocol:"),
		cli.SuccessStyle.Render("LocalSend v2.1"))
}

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
