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
	fmt.Printf("    localgo-cli <COMMAND> [OPTIONS]\n\n")

	fmt.Printf("%s\n", cli.Colorize("COMMANDS:", cli.ColorBold+cli.ColorYellow))
	commands := []struct {
		name, desc string
	}{
		{"serve", "Start the LocalGo server to receive files"},
		{"discover", "Discover devices using multicast"},
		{"scan", "Scan network for devices using HTTP"},
		{"send", "Send a file to another device"},
		{"info", "Show device information"},
		{"help", "Show help information"},
		{"version", "Show version information"},
	}

	for _, cmd := range commands {
		fmt.Printf("    %-12s %s\n", cli.Colorize(cmd.name, cli.ColorGreen), cmd.desc)
	}

	fmt.Printf("\n%s\n", cli.Colorize("OPTIONS:", cli.ColorBold+cli.ColorYellow))
	fmt.Printf("    %s  Show help\n", cli.Colorize("-h, --help", cli.ColorCyan))
	fmt.Printf("    %s  Show version\n\n", cli.Colorize("-v, --version", cli.ColorCyan))

	fmt.Printf("%s\n", cli.Colorize("EXAMPLES:", cli.ColorBold+cli.ColorYellow))
	examples := []string{
		"localgo-cli serve --port 8080 --http",
		"localgo-cli discover --timeout 10",
		"localgo-cli send --file document.pdf --to MyPhone",
		"localgo-cli help send",
	}

	for _, ex := range examples {
		fmt.Printf("    %s\n", cli.Colorize(ex, cli.ColorGreen))
	}

	fmt.Printf("\n%s\n", cli.Colorize("For more information about a specific command, use:", cli.ColorBold))
	fmt.Printf("    localgo-cli help <COMMAND>\n\n")

	fmt.Printf("%s\n", cli.Colorize("ENVIRONMENT VARIABLES:", cli.ColorBold+cli.ColorYellow))
	envVars := []struct {
		name, desc string
	}{
		{"LOCALSEND_ALIAS", "Device alias"},
		{"LOCALSEND_PORT", "Default port"},
		{"LOCALSEND_DOWNLOAD_DIR", "Download directory"},
		{"LOCALSEND_MULTICAST_GROUP", "Multicast group address"},
		{"LOCALSEND_SECURITY_DIR", "Security directory path"},
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
			Usage:       "localgo-cli serve [OPTIONS]",
			Examples: []string{
				"localgo-cli serve",
				"localgo-cli serve --port 8080 --http",
				"localgo-cli serve --pin 123456 --alias MyDevice",
				"localgo-cli serve --dir /tmp/downloads --verbose",
			},
			Flags: []FlagHelp{
				{Name: "--port", Type: "int", Default: "from config", Description: "Port to run the server on"},
				{Name: "--http", Type: "bool", Default: "false", Description: "Use HTTP instead of HTTPS"},
				{Name: "--pin", Type: "string", Default: "", Description: "PIN for authentication"},
				{Name: "--alias", Type: "string", Default: "from config", Description: "Device alias"},
				{Name: "--dir", Type: "string", Default: "from config", Description: "Download directory"},
				{Name: "--quiet", Type: "bool", Default: "false", Description: "Quiet mode - minimal output"},
				{Name: "--verbose", Type: "bool", Default: "false", Description: "Verbose mode - detailed output"},
			},
		},
		"discover": {
			Name:        "discover",
			Description: "Discover LocalGo devices on the network using multicast",
			Usage:       "localgo-cli discover [OPTIONS]",
			Examples: []string{
				"localgo-cli discover",
				"localgo-cli discover --timeout 10",
				"localgo-cli discover --json",
				"localgo-cli discover --quiet",
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
			Usage:       "localgo-cli scan [OPTIONS]",
			Examples: []string{
				"localgo-cli scan",
				"localgo-cli scan --port 8080 --timeout 30",
				"localgo-cli scan --json",
				"localgo-cli scan --quiet",
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
			Usage:       "localgo-cli send --file FILE --to DEVICE [OPTIONS]",
			Examples: []string{
				"localgo-cli send --file document.pdf --to MyPhone",
				"localgo-cli send --file /path/to/file.txt --to 'John's Laptop'",
				"localgo-cli send --file image.jpg --to MyDevice --port 8080",
				"localgo-cli send --file data.zip --to RemotePC --timeout 60",
			},
			Flags: []FlagHelp{
				{Name: "--file", Type: "string", Default: "", Description: "File to send (required)"},
				{Name: "--to", Type: "string", Default: "", Description: "Target device alias (required)"},
				{Name: "--port", Type: "int", Default: "auto-detect", Description: "Target device port"},
				{Name: "--timeout", Type: "int", Default: "30", Description: "Send timeout in seconds"},
				{Name: "--alias", Type: "string", Default: "from config", Description: "Sender alias"},
			},
		},
		"info": {
			Name:        "info",
			Description: "Show device information and configuration",
			Usage:       "localgo-cli info [OPTIONS]",
			Examples: []string{
				"localgo-cli info",
				"localgo-cli info --json",
			},
			Flags: []FlagHelp{
				{Name: "--json", Type: "bool", Default: "false", Description: "Output in JSON format"},
			},
		},
	}

	return commands[commandName]
}
