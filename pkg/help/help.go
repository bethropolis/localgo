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
	header := cli.Colorize("LocalGo CLI", cli.ColorBold+cli.ColorCyan)
	subheader := cli.Colorize("LocalSend v2.1 Protocol Implementation", cli.ColorCyan)

	fmt.Printf("%s - %s\n\n", header, subheader)

	fmt.Printf("%s\n", cli.Colorize("USAGE:", cli.ColorBold+cli.ColorYellow))
	fmt.Printf("    localgo <COMMAND> [OPTIONS]\n\n")

	fmt.Printf("%s\n", cli.Colorize("COMMANDS:", cli.ColorBold+cli.ColorYellow))
	commands := []struct {
		name, desc string
	}{
		{"serve", "Start the LocalGo server to receive files"},
		{"share", "Share files so other devices can download them"},
		{"send", "Send a file to another device"},
		{"discover", "Discover devices using multicast"},
		{"scan", "Scan network for devices using HTTP"},
		{"devices", "List recently discovered devices"},
		{"info", "Show device information"},
		{"help", "Show help information"},
		{"version", "Show version information"},
	}

	for _, cmd := range commands {
		fmt.Printf("    %-12s %s\n", cli.Colorize(cmd.name, cli.ColorGreen), cmd.desc)
	}

	fmt.Printf("\n%s\n", cli.Colorize("OPTIONS:", cli.ColorBold+cli.ColorYellow))
	fmt.Printf("    %s  Show help\n", cli.Colorize("-h, --help", cli.ColorCyan))
	fmt.Printf("    %s  Show version\n", cli.Colorize("-v, --version", cli.ColorCyan))
	fmt.Printf("    %s      Enable debug logging\n", cli.Colorize("--verbose", cli.ColorCyan))
	fmt.Printf("    %s         Enable JSON log output\n\n", cli.Colorize("--json", cli.ColorCyan))

	fmt.Printf("%s\n", cli.Colorize("EXAMPLES:", cli.ColorBold+cli.ColorYellow))
	examples := []string{
		"localgo serve --port 8080 --http",
		"localgo discover --timeout 10",
		"localgo send --file document.pdf --to MyPhone",
		"localgo share --file document.pdf",
		"localgo help send",
	}

	for _, ex := range examples {
		fmt.Printf("    %s\n", cli.Colorize(ex, cli.ColorGreen))
	}

	fmt.Printf("\n%s\n", cli.Colorize("For more information about a specific command, use:", cli.ColorBold))
	fmt.Printf("    localgo help <COMMAND>\n\n")

	fmt.Printf("%s\n", cli.Colorize("ENVIRONMENT VARIABLES:", cli.ColorBold+cli.ColorYellow))
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
		{"LOCALSEND_MULTICAST_GROUP", "Multicast group address"},
		{"LOCALSEND_SECURITY_DIR", "Security directory path"},
		{"LOCALSEND_LOG_LEVEL", "Log verbosity (debug/info/warn/error)"},
	}

	for _, env := range envVars {
		fmt.Printf("    %-28s %s\n", cli.Colorize(env.name, cli.ColorCyan), env.desc)
	}
	fmt.Println()
}

// ShowCommandHelp displays help for a specific command
func ShowCommandHelp(help CommandHelp) {
	header := cli.Colorize("LocalGo CLI", cli.ColorBold+cli.ColorCyan)

	fmt.Printf("%s - %s\n", header, help.Description)
	fmt.Printf("\n%s\n", cli.Colorize("USAGE:", cli.ColorBold+cli.ColorYellow))
	fmt.Printf("    %s\n\n", help.Usage)

	if len(help.Examples) > 0 {
		fmt.Printf("%s\n", cli.Colorize("EXAMPLES:", cli.ColorBold+cli.ColorYellow))
		for _, example := range help.Examples {
			fmt.Printf("    %s\n", cli.Colorize(example, cli.ColorGreen))
		}
		fmt.Println()
	}

	if len(help.Flags) > 0 {
		fmt.Printf("%s\n", cli.Colorize("OPTIONS:", cli.ColorBold+cli.ColorYellow))
		for _, flag := range help.Flags {
			flagName := cli.Colorize(flag.Name, cli.ColorCyan)
			defaultVal := ""
			if flag.Default != "" {
				defaultVal = fmt.Sprintf(" %s", cli.Colorize(fmt.Sprintf("(default: %s)", flag.Default), cli.ColorPurple))
			}
			fmt.Printf("    %-30s %s%s\n", flagName, flag.Description, defaultVal)
		}
		fmt.Println()
	}
}

// ShowVersion displays version information with colors
func ShowVersion(version, commit, date string) {
	fmt.Printf("%s %s\n",
		cli.Colorize("LocalGo CLI", cli.ColorBold+cli.ColorCyan),
		cli.Colorize(version, cli.ColorGreen))
	fmt.Printf("%s %s\n",
		cli.Colorize("Git Commit:", cli.ColorYellow),
		commit)
	fmt.Printf("%s %s\n",
		cli.Colorize("Build Date:", cli.ColorYellow),
		date)
	fmt.Printf("%s %s\n",
		cli.Colorize("Protocol:", cli.ColorYellow),
		cli.Colorize("LocalSend v2.1", cli.ColorGreen))
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
				{Name: "--quiet", Type: "bool", Default: "false", Description: "Quiet mode - minimal output"},
				{Name: "--verbose", Type: "bool", Default: "false", Description: "Verbose mode - detailed output"},
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
			},
			Flags: []FlagHelp{
				{Name: "--file", Type: "string", Default: "", Description: "File or directory to share (required, can be specified multiple times)"},
				{Name: "--port", Type: "int", Default: "from config", Description: "Port to run the server on"},
				{Name: "--http", Type: "bool", Default: "false", Description: "Use HTTP instead of HTTPS"},
				{Name: "--pin", Type: "string", Default: "", Description: "PIN for authentication"},
				{Name: "--alias", Type: "string", Default: "from config", Description: "Device alias"},
				{Name: "--auto-accept", Type: "bool", Default: "false", Description: "Auto-accept incoming files without prompting"},
				{Name: "--no-clipboard", Type: "bool", Default: "false", Description: "Save incoming text as a file instead of copying to clipboard"},
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
				{Name: "--timeout", Type: "int", Default: "5", Description: "Discovery timeout in seconds"},
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
			},
			Flags: []FlagHelp{
				{Name: "--timeout", Type: "int", Default: "15", Description: "Scan timeout in seconds"},
				{Name: "--port", Type: "int", Default: "from config", Description: "Port to scan"},
				{Name: "--json", Type: "bool", Default: "false", Description: "Output in JSON format"},
				{Name: "--quiet", Type: "bool", Default: "false", Description: "Quiet mode - only show results"},
			},
		},
		"send": {
			Name:        "send",
			Description: "Send a file to another LocalGo device",
			Usage:       "localgo send --file FILE --to DEVICE [OPTIONS]",
			Examples: []string{
				"localgo send --file document.pdf --to MyPhone",
				"localgo send --file /path/to/file.txt --to 'John's Laptop'",
				"localgo send --file image.jpg --file text.txt --to MyDevice",
				"localgo send --file data.zip --to RemotePC --timeout 60",
			},
			Flags: []FlagHelp{
				{Name: "--file", Type: "string", Default: "", Description: "File or directory to send (required, can be specified multiple times)"},
				{Name: "--to", Type: "string", Default: "", Description: "Target device alias (required)"},
				{Name: "--port", Type: "int", Default: "auto-detect", Description: "Target device port"},
				{Name: "--timeout", Type: "int", Default: "30", Description: "Send timeout in seconds"},
				{Name: "--alias", Type: "string", Default: "from config", Description: "Sender alias"},
			},
		},
		"devices": {
			Name:        "devices",
			Description: "List recently discovered devices on the network",
			Usage:       "localgo devices [OPTIONS]",
			Examples: []string{
				"localgo devices",
				"localgo devices --json",
			},
			Flags: []FlagHelp{
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
	}

	return commands[commandName]
}
