package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/logging"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/bethropolis/localgo/pkg/send"
	"github.com/bethropolis/localgo/pkg/server"
	"github.com/sirupsen/logrus"
)

// Version information (can be set during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// Command represents a CLI command
type Command struct {
	Name        string
	Description string
	Usage       string
	Examples    []string
	Flags       *flag.FlagSet
	Action      func(cfg *config.Config, args []string) error
}

// Application holds the CLI application state
type Application struct {
	commands map[string]*Command
	cfg      *config.Config
}

func main() {
	app := &Application{
		commands: make(map[string]*Command),
	}

	// Initialize logging first
	logging.Init()

	// Register all commands
	app.registerCommands()

	// Parse arguments
	if len(os.Args) < 2 {
		app.showUsage()
		os.Exit(1)
	}

	commandName := os.Args[1]

	// Handle special commands
	switch commandName {
	case "help", "-h", "--help":
		if len(os.Args) > 2 {
			app.showCommandHelp(os.Args[2])
		} else {
			app.showUsage()
		}
		return
	case "version", "-v", "--version":
		app.showVersion()
		return
	}

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %v", err)
	}
	if cfg.SecurityContext == nil {
		logrus.Fatalf("Security context is missing after loading config")
	}
	app.cfg = cfg

	// Find and execute command
	cmd, exists := app.commands[commandName]
	if !exists {
		logrus.Errorf("Unknown command: %s", commandName)
		app.showUsage()
		os.Exit(1)
	}

	// Execute command
	if err := cmd.Action(cfg, os.Args[2:]); err != nil {
		logrus.Fatalf("Command failed: %v", err)
	}
}

func (app *Application) registerCommands() {
	// Serve command
	serveFlags := flag.NewFlagSet("serve", flag.ExitOnError)
	servePort := serveFlags.Int("port", 0, "Port to run the server on (default: from config)")
	serveHTTP := serveFlags.Bool("http", false, "Use HTTP instead of HTTPS")
	servePin := serveFlags.String("pin", "", "PIN for authentication")
	serveAlias := serveFlags.String("alias", "", "Device alias (default: from config)")
	serveDir := serveFlags.String("dir", "", "Download directory (default: from config)")
	serveQuiet := serveFlags.Bool("quiet", false, "Quiet mode - minimal output")
	serveVerbose := serveFlags.Bool("verbose", false, "Verbose mode - detailed output")

	app.commands["serve"] = &Command{
		Name:        "serve",
		Description: "Start the LocalGo server to receive files",
		Usage:       "localgo-cli serve [OPTIONS]",
		Examples: []string{
			"localgo-cli serve",
			"localgo-cli serve --port 8080 --http",
			"localgo-cli serve --pin 123456 --alias MyDevice",
			"localgo-cli serve --dir /tmp/downloads --verbose",
		},
		Flags: serveFlags,
		Action: func(cfg *config.Config, args []string) error {
			serveFlags.Parse(args)
			return app.runServe(cfg, servePort, serveHTTP, servePin, serveAlias, serveDir, serveQuiet, serveVerbose)
		},
	}

	// Discover command
	discoverFlags := flag.NewFlagSet("discover", flag.ExitOnError)
	discoverTimeout := discoverFlags.Int("timeout", 5, "Discovery timeout in seconds")
	discoverJSON := discoverFlags.Bool("json", false, "Output in JSON format")
	discoverQuiet := discoverFlags.Bool("quiet", false, "Quiet mode - only show results")

	app.commands["discover"] = &Command{
		Name:        "discover",
		Description: "Discover LocalGo devices on the network using multicast",
		Usage:       "localgo-cli discover [OPTIONS]",
		Examples: []string{
			"localgo-cli discover",
			"localgo-cli discover --timeout 10",
			"localgo-cli discover --json",
			"localgo-cli discover --quiet",
		},
		Flags: discoverFlags,
		Action: func(cfg *config.Config, args []string) error {
			discoverFlags.Parse(args)
			return app.runDiscover(cfg, discoverTimeout, discoverJSON, discoverQuiet)
		},
	}

	// Scan command
	scanFlags := flag.NewFlagSet("scan", flag.ExitOnError)
	scanTimeout := scanFlags.Int("timeout", 15, "Scan timeout in seconds")
	scanPort := scanFlags.Int("port", 0, "Port to scan (default: from config)")
	scanJSON := scanFlags.Bool("json", false, "Output in JSON format")
	scanQuiet := scanFlags.Bool("quiet", false, "Quiet mode - only show results")

	app.commands["scan"] = &Command{
		Name:        "scan",
		Description: "Scan the network for LocalGo devices using HTTP",
		Usage:       "localgo-cli scan [OPTIONS]",
		Examples: []string{
			"localgo-cli scan",
			"localgo-cli scan --port 8080 --timeout 30",
			"localgo-cli scan --json",
			"localgo-cli scan --quiet",
		},
		Flags: scanFlags,
		Action: func(cfg *config.Config, args []string) error {
			scanFlags.Parse(args)
			return app.runScan(cfg, scanTimeout, scanPort, scanJSON, scanQuiet)
		},
	}

	// Send command
	sendFlags := flag.NewFlagSet("send", flag.ExitOnError)
	sendFile := sendFlags.String("file", "", "File to send (required)")
	sendTo := sendFlags.String("to", "", "Target device alias (required)")
	sendPort := sendFlags.Int("port", 0, "Target device port (default: auto-detect)")
	sendTimeout := sendFlags.Int("timeout", 30, "Send timeout in seconds")
	sendAlias := sendFlags.String("alias", "", "Sender alias (default: from config)")

	app.commands["send"] = &Command{
		Name:        "send",
		Description: "Send a file to another LocalGo device",
		Usage:       "localgo-cli send --file FILE --to DEVICE [OPTIONS]",
		Examples: []string{
			"localgo-cli send --file document.pdf --to MyPhone",
			"localgo-cli send --file /path/to/file.txt --to 'John\\'s Laptop'",
			"localgo-cli send --file image.jpg --to MyDevice --port 8080",
			"localgo-cli send --file data.zip --to RemotePC --timeout 60",
		},
		Flags: sendFlags,
		Action: func(cfg *config.Config, args []string) error {
			sendFlags.Parse(args)
			return app.runSend(cfg, sendFile, sendTo, sendPort, sendTimeout, sendAlias)
		},
	}

	// Info command
	infoFlags := flag.NewFlagSet("info", flag.ExitOnError)
	infoJSON := infoFlags.Bool("json", false, "Output in JSON format")

	app.commands["info"] = &Command{
		Name:        "info",
		Description: "Show device information and configuration",
		Usage:       "localgo-cli info [OPTIONS]",
		Examples: []string{
			"localgo-cli info",
			"localgo-cli info --json",
		},
		Flags: infoFlags,
		Action: func(cfg *config.Config, args []string) error {
			infoFlags.Parse(args)
			return app.runInfo(cfg, infoJSON)
		},
	}
}

func (app *Application) showUsage() {
	fmt.Printf(`LocalGo CLI - LocalSend v2.1 Protocol Implementation

USAGE:
    localgo-cli <COMMAND> [OPTIONS]

COMMANDS:
    serve      Start the LocalGo server to receive files
    discover   Discover devices using multicast
    scan       Scan network for devices using HTTP
    send       Send a file to another device
    info       Show device information
    help       Show help information
    version    Show version information

OPTIONS:
    -h, --help     Show help
    -v, --version  Show version

EXAMPLES:
    localgo-cli serve --port 8080 --http
    localgo-cli discover --timeout 10
    localgo-cli send --file document.pdf --to MyPhone
    localgo-cli help send

For more information about a specific command, use:
    localgo-cli help <COMMAND>

Environment Variables:
    LOCALSEND_ALIAS          Device alias
    LOCALSEND_PORT           Default port
    LOCALSEND_DOWNLOAD_DIR   Download directory
    LOCALSEND_MULTICAST_GROUP Multicast group address

`)
}

func (app *Application) showCommandHelp(commandName string) {
	cmd, exists := app.commands[commandName]
	if !exists {
		fmt.Printf("Unknown command: %s\n", commandName)
		app.showUsage()
		return
	}

	fmt.Printf("LocalGo CLI - %s\n\n", cmd.Description)
	fmt.Printf("USAGE:\n    %s\n\n", cmd.Usage)

	if len(cmd.Examples) > 0 {
		fmt.Printf("EXAMPLES:\n")
		for _, example := range cmd.Examples {
			fmt.Printf("    %s\n", example)
		}
		fmt.Printf("\n")
	}

	fmt.Printf("OPTIONS:\n")
	cmd.Flags.PrintDefaults()
}

func (app *Application) showVersion() {
	fmt.Printf("LocalGo CLI %s\n", Version)
	fmt.Printf("Git Commit: %s\n", GitCommit)
	fmt.Printf("Build Date: %s\n", BuildDate)
	fmt.Printf("Protocol: LocalSend v2.1\n")
}

func (app *Application) runServe(cfg *config.Config, port *int, useHTTP *bool, pin *string, alias *string, dir *string, quiet *bool, verbose *bool) error {
	// Set log level
	if *quiet {
		logrus.SetLevel(logrus.WarnLevel)
	} else if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Apply overrides
	if *port > 0 {
		cfg.Port = *port
	}
	if *useHTTP {
		cfg.HttpsEnabled = false
	}
	if *pin != "" {
		cfg.PIN = *pin
	}
	if *alias != "" {
		cfg.Alias = *alias
	}
	if *dir != "" {
		cfg.DownloadDir = *dir
	}

	// Create download directory if it doesn't exist
	if err := os.MkdirAll(cfg.DownloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	// Show startup information
	protocol := "HTTPS"
	if !cfg.HttpsEnabled {
		protocol = "HTTP"
	}

	logrus.Infof("Starting LocalGo server")
	logrus.Infof("  Alias: %s", cfg.Alias)
	logrus.Infof("  Protocol: %s", protocol)
	logrus.Infof("  Port: %d", cfg.Port)
	logrus.Infof("  Download Directory: %s", cfg.DownloadDir)
	if cfg.PIN != "" {
		logrus.Infof("  PIN Protection: Enabled")
	}
	logrus.Infof("  Fingerprint: %s", cfg.SecurityContext.CertificateHash[:16]+"...")

	// Context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Initialize discovery service
	discoverySvcConfig := discovery.DefaultServiceConfig()
	discoverySvcConfig.MulticastConfig.Port = cfg.Port
	discoverySvcConfig.MulticastConfig.MulticastAddr = fmt.Sprintf("%s:%d", cfg.MulticastGroup, cfg.Port)

	protocol_type := model.ProtocolTypeHTTP
	if cfg.HttpsEnabled {
		protocol_type = model.ProtocolTypeHTTPS
	}

	multicastDto := model.MulticastDto{
		Alias:       cfg.Alias,
		Version:     config.ProtocolVersion,
		DeviceModel: cfg.DeviceModel,
		DeviceType:  cfg.DeviceType,
		Fingerprint: cfg.SecurityContext.CertificateHash,
		Port:        cfg.Port,
		Protocol:    protocol_type,
		Download:    false,
		Announce:    true,
	}

	multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto)
	discoverySvc := discovery.NewService(discoverySvcConfig, multicast)

	discoverySvc.AddDeviceHandler(func(device *model.Device) {
		logrus.Infof("Device discovered: %s (%s)", device.Alias, device.IP)
	})

	// Start discovery service
	go func() {
		err := discoverySvc.Start(ctx, cfg.Alias, cfg.Port, cfg.SecurityContext.CertificateHash, cfg.DeviceType, cfg.DeviceModel)
		if err != nil {
			logrus.Errorf("Discovery service failed: %v", err)
		}
	}()

	// Start server
	srv := server.NewServer(cfg)

	logrus.Infof("Server ready! Waiting for files...")
	logrus.Infof("Press Ctrl+C to stop")

	err := srv.Start(ctx)
	if err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	discoverySvc.Stop()
	logrus.Infof("Server stopped")
	return nil
}

func (app *Application) runDiscover(cfg *config.Config, timeout *int, jsonOutput *bool, quiet *bool) error {
	if *quiet {
		logrus.SetLevel(logrus.WarnLevel)
	}

	// Increase default timeout for better reliability
	discoverTimeout := *timeout
	if discoverTimeout < 10 {
		discoverTimeout = 10
	}

	if !*quiet {
		logrus.Infof("Discovering devices (timeout: %ds)...", discoverTimeout)
		logrus.Infof("  Multicast group: %s", cfg.MulticastGroup)
		logrus.Infof("  Port: %d", cfg.Port)
		logrus.Infof("  Protocol: %s", func() string {
			if cfg.HttpsEnabled {
				return "HTTPS"
			}
			return "HTTP"
		}())
	}

	// Initialize discovery service
	discoverySvcConfig := discovery.DefaultServiceConfig()
	discoverySvcConfig.MulticastConfig.Port = cfg.Port
	discoverySvcConfig.MulticastConfig.MulticastAddr = fmt.Sprintf("%s:%d", cfg.MulticastGroup, cfg.Port)

	protocol := model.ProtocolTypeHTTP
	if cfg.HttpsEnabled {
		protocol = model.ProtocolTypeHTTPS
	}

	multicastDto := model.MulticastDto{
		Alias:       cfg.Alias,
		Version:     config.ProtocolVersion,
		DeviceModel: cfg.DeviceModel,
		DeviceType:  cfg.DeviceType,
		Fingerprint: cfg.SecurityContext.CertificateHash,
		Port:        cfg.Port,
		Protocol:    protocol,
		Download:    false,
		Announce:    true,
	}

	multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto)
	discoverySvc := discovery.NewService(discoverySvcConfig, multicast)

	discoverySvc.AddDeviceHandler(func(device *model.Device) {
		if !*quiet {
			logrus.Infof("Found: %s (%s) [%s] Port: %d", device.Alias, device.IP, device.Protocol, device.Port)
		}
	})

	// Perform discovery
	discoverCtx, cancel := context.WithTimeout(context.Background(), time.Duration(discoverTimeout)*time.Second)
	defer cancel()

	foundDevices, err := discoverySvc.Discover(discoverCtx, cfg.Alias, cfg.Port, cfg.SecurityContext.CertificateHash, cfg.DeviceType, cfg.DeviceModel)
	if err != nil && !*quiet {
		logrus.Warnf("Discovery completed with warnings: %v", err)
	}

	if !*quiet && len(foundDevices) == 0 {
		logrus.Warnf("No devices discovered. If you expected to see a device, check:\n- That both devices are on the same Wi-Fi network\n- That firewalls are not blocking UDP port %d\n- That AP/Client Isolation is disabled on your router\n- That the LocalSend app is open and in receive mode", cfg.Port)
	}

	return app.displayDevices(foundDevices, *jsonOutput, *quiet, "multicast discovery")
}

func (app *Application) runScan(cfg *config.Config, timeout *int, port *int, jsonOutput *bool, quiet *bool) error {
	if *quiet {
		logrus.SetLevel(logrus.WarnLevel)
	}

	// Increase default timeout for better reliability
	scanTimeout := *timeout
	if scanTimeout < 15 {
		scanTimeout = 15
	}

	scanPort := cfg.Port
	if *port > 0 {
		scanPort = *port
	}

	// Get local IPs
	ips, err := network.GetLocalIPAddresses()
	if err != nil {
		return fmt.Errorf("failed to get local network IPs: %w", err)
	}

	if !*quiet {
		logrus.Infof("Scanning network on port %d (timeout: %ds)...", scanPort, scanTimeout)
		logrus.Infof("  Scanning %d IP addresses...", len(ips))
		logrus.Infof("  Protocols: HTTPS first, then HTTP fallback")
	}

	// Initialize HTTP discovery
	httpDiscoverer := discovery.NewHTTPDiscovery(nil, cfg.ToRegisterDto(), nil)

	// Perform scan
	scanCtx, cancel := context.WithTimeout(context.Background(), time.Duration(scanTimeout)*time.Second)
	defer cancel()

	foundDevices, err := httpDiscoverer.ScanNetwork(scanCtx, ips, scanPort)
	if err != nil && !*quiet {
		logrus.Warnf("Scan completed with warnings: %v", err)
	}

	if !*quiet && len(foundDevices) == 0 {
		logrus.Warnf("No devices found during scan. If you expected to see a device, check:\n- That both devices are on the same Wi-Fi network\n- That firewalls are not blocking TCP ports %d (HTTP/HTTPS)\n- That AP/Client Isolation is disabled on your router\n- That the LocalSend app is open and in receive mode", scanPort)
	}

	return app.displayDevices(foundDevices, *jsonOutput, *quiet, "HTTP scan")
}

func (app *Application) runSend(cfg *config.Config, file *string, to *string, port *int, timeout *int, alias *string) error {
	// Validate required parameters
	if *file == "" {
		return fmt.Errorf("file parameter is required (use --file)")
	}
	if *to == "" {
		return fmt.Errorf("target device parameter is required (use --to)")
	}

	// Check if file exists
	if _, err := os.Stat(*file); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", *file)
	}

	// Apply overrides
	if *alias != "" {
		cfg.Alias = *alias
	}

	// Get file info for display
	fileInfo, err := os.Stat(*file)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	logrus.Infof("Sending file: %s", filepath.Base(*file))
	logrus.Infof("  Size: %s", cli.FormatBytes(fileInfo.Size()))
	logrus.Infof("  To: %s", *to)
	logrus.Infof("  From: %s", cfg.Alias)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// Send file
	err = send.SendFile(ctx, cfg, *file, *to, *port)
	if err != nil {
		return fmt.Errorf("failed to send file: %w", err)
	}

	logrus.Infof("File sent successfully!")
	return nil
}

func (app *Application) runInfo(cfg *config.Config, jsonOutput *bool) error {
	format := cli.FormatTable
	if *jsonOutput {
		format = cli.FormatJSON
	}

	writer := cli.NewOutputWriter(format)
	defer writer.Flush()

	deviceModel := "Unknown"
	if cfg.DeviceModel != nil {
		deviceModel = *cfg.DeviceModel
	}

	protocol := "HTTP"
	if cfg.HttpsEnabled {
		protocol = "HTTPS"
	}

	info := cli.DeviceInfo{
		Alias:         cfg.Alias,
		Version:       config.ProtocolVersion,
		DeviceModel:   deviceModel,
		DeviceType:    string(cfg.DeviceType),
		Fingerprint:   cfg.SecurityContext.CertificateHash,
		Port:          cfg.Port,
		Protocol:      protocol,
		DownloadDir:   cfg.DownloadDir,
		HasPin:        cfg.PIN != "",
		MulticastAddr: fmt.Sprintf("%s:%d", cfg.MulticastGroup, cfg.Port),
	}

	return writer.WriteDeviceInfo(info)
}

func (app *Application) displayDevices(devices []*model.Device, jsonOutput bool, quiet bool, method string) error {
	format := cli.FormatTable
	if jsonOutput {
		format = cli.FormatJSON
	} else if quiet {
		format = cli.FormatQuiet
	}

	writer := cli.NewOutputWriter(format)
	defer writer.Flush()

	return writer.WriteDevices(devices, method)
}

// Helper functions moved to cli package
