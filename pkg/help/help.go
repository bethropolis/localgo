package help

import (
	"fmt"
	"strings"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/charmbracelet/lipgloss"
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

	maxCmdWidth := 0
	for _, cmd := range commands {
		if w := lipgloss.Width(cmd.name); w > maxCmdWidth {
			maxCmdWidth = w
		}
	}
	cmdPad := maxCmdWidth + 2

	for _, cmd := range commands {
		styledName := cli.SuccessStyle.Render(cmd.name)
		padding := cmdPad - lipgloss.Width(cmd.name)
		fmt.Printf("    %s%s%s\n", styledName, strings.Repeat(" ", padding), cmd.desc)
	}

	fmt.Printf("\n%s\n", cli.WarningStyle.Render("OPTIONS:"))
	options := []struct{ flag, desc string }{
		{"-h, --help", "Show help"},
		{"-v, --version", "Show version"},
		{"--verbose", "Enable debug logging"},
		{"--json", "Enable JSON log output"},
		{"--private, -p", "Hide device identity during discovery/transfer"},
		{"--config", "Config file path"},
	}

	maxOptWidth := 0
	for _, opt := range options {
		if w := lipgloss.Width(opt.flag); w > maxOptWidth {
			maxOptWidth = w
		}
	}
	optPad := maxOptWidth + 2

	for _, opt := range options {
		styledFlag := cli.InfoStyle.Render(opt.flag)
		padding := optPad - lipgloss.Width(opt.flag)
		fmt.Printf("    %s%s%s\n", styledFlag, strings.Repeat(" ", padding), opt.desc)
	}
	fmt.Println()

	fmt.Printf("%s\n", cli.WarningStyle.Render("EXAMPLES:"))
	examples := []string{
		"localgo serve --port 8080 --http",
		"localgo discover --timeout 10",
		"localgo send --file document.pdf --to MyPhone",
		"localgo send --clipboard --to MyPhone",
		"localgo send --stdin < document.txt --to MyPhone",
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


