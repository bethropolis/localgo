package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/logging"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/bethropolis/localgo/pkg/send"
	"github.com/bethropolis/localgo/pkg/server"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Version information (can be set during build)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// StringSliceFlag is a custom flag type that allows multiple occurrences
type StringSliceFlag []string

func (s *StringSliceFlag) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *StringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

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

	// Pre-parse global flags before subcommand dispatch so the logger is
	// configured correctly from the very first log line.
	// Global flags: --verbose / --json. They may appear anywhere in os.Args.
	// We build a filtered args slice (globals stripped) for command resolution,
	// but leave os.Args untouched so per-subcommand flag.FlagSet parsing still
	// sees all arguments (subcommand flag sets ignore unknown flags gracefully).
	globalVerbose := false
	globalJSON := false
	filteredArgs := make([]string, 0, len(os.Args))
	filteredArgs = append(filteredArgs, os.Args[0]) // program name
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--verbose", "-verbose":
			globalVerbose = true
		case "--json", "-json":
			globalJSON = true
		default:
			filteredArgs = append(filteredArgs, arg)
		}
	}

	// Initialize logging first
	logger := logging.Init(globalVerbose, globalJSON)

	// Register all commands
	app.registerCommands()

	// Parse arguments (use filteredArgs so global flags don't shadow the command)
	if len(filteredArgs) < 2 {
		help.ShowMainUsage()
		os.Exit(1)
	}

	commandName := filteredArgs[1]

	// Handle special commands
	switch commandName {
	case "help", "-h", "--help":
		if len(filteredArgs) > 2 {
			if cmdHelp := help.GetCommandHelp(filteredArgs[2]); cmdHelp != nil {
				help.ShowCommandHelp(*cmdHelp)
			} else {
				fmt.Printf("Unknown command: %s\n", filteredArgs[2])
				help.ShowMainUsage()
			}
		} else {
			help.ShowMainUsage()
		}
		return
	case "version", "-v", "--version":
		help.ShowVersion(Version, GitCommit, BuildDate)
		return
	}

	// Load configuration
	cfg, err := config.LoadConfig(logger)
	if err != nil {
		zap.S().Fatalf("Failed to load configuration: %v", err)
	}
	if cfg.SecurityContext == nil {
		zap.S().Fatalf("Security context is missing after loading config")
	}
	app.cfg = cfg

	// Find and execute command
	cmd, exists := app.commands[commandName]
	if !exists {
		zap.S().Errorf("Unknown command: %s", commandName)
		help.ShowMainUsage()
		os.Exit(1)
	}

	// Execute command (pass subcommand args without the global flags)
	if err := cmd.Action(cfg, filteredArgs[2:]); err != nil {
		zap.S().Fatalf("Command failed: %v", err)
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
	serveInterval := serveFlags.Int("interval", 30, "Discovery announcement interval in seconds")
	serveAutoAccept := serveFlags.Bool("auto-accept", false, "Auto-accept incoming files without prompting")
	serveNoClipboard := serveFlags.Bool("no-clipboard", false, "Save incoming text as a file instead of copying to clipboard")
	serveHistory := serveFlags.String("history", "", "Path to transfer history JSONL file (default: ~/.local/share/localgo/history.jsonl).\n\t\t\t\tPass 'off' to disable history logging.")
	serveExec := serveFlags.String("exec", "", "Shell command to run after each received file.\n\t\t\t\tAvailable env vars: LOCALGO_FILE, LOCALGO_NAME, LOCALGO_SIZE, LOCALGO_ALIAS, LOCALGO_IP")

	app.commands["serve"] = &Command{
		Name:        "serve",
		Description: "Start the LocalGo server to receive files",
		Usage:       "localgo serve [OPTIONS]",
		Examples: []string{
			"localgo serve",
			"localgo serve --port 8080 --http",
			"localgo serve --pin 123456 --alias MyDevice",
			"localgo serve --dir /tmp/downloads --verbose",
			"localgo serve --quiet --interval 300",
			"localgo serve --auto-accept",
		},
		Flags: serveFlags,
		Action: func(cfg *config.Config, args []string) error {
			serveFlags.Parse(args)
			return app.runServe(cfg, servePort, serveHTTP, servePin, serveAlias, serveDir, serveQuiet, serveVerbose, serveInterval, serveAutoAccept, serveNoClipboard, serveHistory, serveExec)
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
		Usage:       "localgo discover [OPTIONS]",
		Examples: []string{
			"localgo discover",
			"localgo discover --timeout 10",
			"localgo discover --json",
			"localgo discover --quiet",
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
		Usage:       "localgo scan [OPTIONS]",
		Examples: []string{
			"localgo scan",
			"localgo scan --port 8080 --timeout 30",
			"localgo scan --json",
			"localgo scan --quiet",
		},
		Flags: scanFlags,
		Action: func(cfg *config.Config, args []string) error {
			scanFlags.Parse(args)
			return app.runScan(cfg, scanTimeout, scanPort, scanJSON, scanQuiet)
		},
	}

	// Send command
	sendFlags := flag.NewFlagSet("send", flag.ExitOnError)
	var sendFiles StringSliceFlag
	sendFlags.Var(&sendFiles, "file", "File or directory to send (can be specified multiple times)")
	sendTo := sendFlags.String("to", "", "Target device alias (required)")
	sendPort := sendFlags.Int("port", 0, "Target device port (default: auto-detect)")
	sendTimeout := sendFlags.Int("timeout", 30, "Send timeout in seconds")
	sendAlias := sendFlags.String("alias", "", "Sender alias (default: from config)")

	app.commands["send"] = &Command{
		Name:        "send",
		Description: "Send a file to another LocalGo device",
		Usage:       "localgo send --file FILE --to DEVICE [OPTIONS]",
		Examples: []string{
			"localgo send --file document.pdf --to MyPhone",
			"localgo send --file /path/to/file.txt --to 'John\\'s Laptop'",
			"localgo send --file image.jpg --file text.txt --to MyDevice",
			"localgo send --file data.zip --to RemotePC --timeout 60",
		},
		Flags: sendFlags,
		Action: func(cfg *config.Config, args []string) error {
			sendFlags.Parse(args)
			return app.runSend(cfg, sendFiles, sendTo, sendPort, sendTimeout, sendAlias)
		},
	}

	// Share command
	shareFlags := flag.NewFlagSet("share", flag.ExitOnError)
	var shareFiles StringSliceFlag
	shareFlags.Var(&shareFiles, "file", "File or directory to share (can be specified multiple times)")
	sharePort := shareFlags.Int("port", 0, "Port to run the server on (default: from config)")
	shareHTTP := shareFlags.Bool("http", false, "Use HTTP instead of HTTPS")
	sharePin := shareFlags.String("pin", "", "PIN for authentication")
	shareAlias := shareFlags.String("alias", "", "Device alias (default: from config)")
	shareAutoAccept := shareFlags.Bool("auto-accept", false, "Auto-accept incoming files without prompting")
	shareNoClipboard := shareFlags.Bool("no-clipboard", false, "Save incoming text as a file instead of copying to clipboard")
	shareHistory := shareFlags.String("history", "", "Path to transfer history JSONL file (default: ~/.local/share/localgo/history.jsonl).\n\t\t\t\tPass 'off' to disable history logging.")
	shareExec := shareFlags.String("exec", "", "Shell command to run after each received file.\n\t\t\t\tAvailable env vars: LOCALGO_FILE, LOCALGO_NAME, LOCALGO_SIZE, LOCALGO_ALIAS, LOCALGO_IP")
	shareQuiet := shareFlags.Bool("quiet", false, "Quiet mode - minimal output")

	app.commands["share"] = &Command{
		Name:        "share",
		Description: "Share files so others can download them",
		Usage:       "localgo share --file FILE [OPTIONS]",
		Examples: []string{
			"localgo share --file document.pdf",
			"localgo share --file image.jpg --file text.txt",
			"localgo share --file data.zip --pin 1234",
			"localgo share --file data.zip --auto-accept",
		},
		Flags: shareFlags,
		Action: func(cfg *config.Config, args []string) error {
			shareFlags.Parse(args)
			return app.runShare(cfg, shareFiles, sharePort, shareHTTP, sharePin, shareAlias, shareAutoAccept, shareNoClipboard, shareHistory, shareExec, shareQuiet)
		},
	}

	// Info command
	infoFlags := flag.NewFlagSet("info", flag.ExitOnError)
	infoJSON := infoFlags.Bool("json", false, "Output in JSON format")

	app.commands["info"] = &Command{
		Name:        "info",
		Description: "Show device information and configuration",
		Usage:       "localgo info [OPTIONS]",
		Examples: []string{
			"localgo info",
			"localgo info --json",
		},
		Flags: infoFlags,
		Action: func(cfg *config.Config, args []string) error {
			infoFlags.Parse(args)
			return app.runInfo(cfg, infoJSON)
		},
	}

	// Devices command
	devicesFlags := flag.NewFlagSet("devices", flag.ExitOnError)
	devicesJSON := devicesFlags.Bool("json", false, "Output in JSON format")

	app.commands["devices"] = &Command{
		Name:        "devices",
		Description: "Show all recently discovered devices on the network",
		Usage:       "localgo devices [OPTIONS]",
		Examples: []string{
			"localgo devices",
			"localgo devices --json",
		},
		Flags: devicesFlags,
		Action: func(cfg *config.Config, args []string) error {
			devicesFlags.Parse(args)
			// For CLI command 'devices', run a quick 2s discovery instead of trying to query server
			// This avoids needing complex IPC with the background server process
			timeout := 2
			quiet := true
			return app.runDiscover(cfg, &timeout, devicesJSON, &quiet)
		},
	}
}

// Help methods removed - now using pkg/help module

func (app *Application) runServe(cfg *config.Config, port *int, useHTTP *bool, pin *string, alias *string, dir *string, quiet *bool, verbose *bool, interval *int, autoAccept *bool, noClipboard *bool, history *string, execHook *string) error {
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
	if *autoAccept {
		cfg.AutoAccept = true
	}
	if *noClipboard {
		cfg.NoClipboard = true
	}
	if *history != "" {
		cfg.HistoryFile = *history
	}
	if *execHook != "" {
		cfg.ExecHook = *execHook
	}
	if *quiet {
		cfg.Quiet = true
	}

	// Create download directory if it doesn't exist
	if err := os.MkdirAll(cfg.DownloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	protocol := "HTTPS"
	if !cfg.HttpsEnabled {
		protocol = "HTTP"
	}

	logFn := zap.S().Infof
	if *quiet {
		logFn = zap.S().Warnf
	}

	logFn("Starting LocalGo server")
	logFn("  Alias: %s", cfg.Alias)
	logFn("  Protocol: %s", protocol)
	logFn("  Port: %d", cfg.Port)
	logFn("  Download Directory: %s", cfg.DownloadDir)
	if cfg.PIN != "" {
		logFn("  PIN Protection: Enabled")
	}
	logFn("  Fingerprint: %s", cfg.SecurityContext.CertificateHash[:16]+"...")

	// Context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Initialize discovery service
	discoverySvcConfig := discovery.DefaultServiceConfig()
	discoverySvcConfig.MulticastConfig.Port = cfg.Port
	discoverySvcConfig.MulticastConfig.MulticastAddr = fmt.Sprintf("%s:%d", cfg.MulticastGroup, cfg.Port)

	if *interval > 0 {
		discoverySvcConfig.AnnounceInterval = time.Duration(*interval) * time.Second
	}

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

	multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto, zap.S())

	// Create HTTPDiscoverer for backchannel (HTTP response to multicast)
	httpDiscoverer := discovery.NewHTTPDiscovery(nil, cfg.ToRegisterDto(), nil, zap.S())
	multicast.SetHTTPDiscoverer(httpDiscoverer)

	discoverySvc := discovery.NewService(discoverySvcConfig, multicast, zap.S())

	discoverySvc.AddDeviceHandler(func(device *model.Device) {
		if !*quiet {
			zap.S().Infof("Device discovered: %s (%s)", device.Alias, device.IP)
		}
	})

	// Start server first
	srv := server.NewServer(cfg, zap.S())

	serverErrChan := make(chan error, 1)
	serverReadyChan := make(chan struct{}, 1)
	go func() {
		serverErrChan <- srv.Start(ctx, serverReadyChan)
	}()

	// Wait for server to be ready (server.Start waits for port bind)
	select {
	case err := <-serverErrChan:
		return fmt.Errorf("server failed: %w", err)
	case <-serverReadyChan:
	}

	// Start discovery AFTER server is ready
	err := discoverySvc.Start(ctx, cfg.Alias, cfg.Port, cfg.SecurityContext.CertificateHash, cfg.DeviceType, cfg.DeviceModel, cfg.HttpsEnabled)
	if err != nil {
		return fmt.Errorf("discovery service failed: %w", err)
	}

	if !*quiet {
		zap.S().Infof("Server ready! Waiting for files...")
		zap.S().Infof("Press Ctrl+C to stop")
	}

	// Wait for server to finish
	if err := <-serverErrChan; err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	discoverySvc.Stop()
	if *quiet {
		zap.S().Warnf("Server stopped")
	} else {
		zap.S().Infof("Server stopped")
	}
	return nil
}

func (app *Application) runShare(cfg *config.Config, files []string, port *int, useHTTP *bool, pin *string, alias *string, autoAccept *bool, noClipboard *bool, history *string, execHook *string, quiet *bool) error {
	if len(files) == 0 {
		return fmt.Errorf("file parameter is required (use --file)")
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
	if *autoAccept {
		cfg.AutoAccept = true
	}
	if *noClipboard {
		cfg.NoClipboard = true
	}
	if *history != "" {
		cfg.HistoryFile = *history
	}
	if *execHook != "" {
		cfg.ExecHook = *execHook
	}
	if *quiet {
		cfg.Quiet = true
	}

	protocol := "HTTPS"
	if !cfg.HttpsEnabled {
		protocol = "HTTP"
	}

	zap.S().Infof("Starting LocalGo Web Share")
	zap.S().Infof("  Alias: %s", cfg.Alias)
	zap.S().Infof("  Protocol: %s", protocol)
	zap.S().Infof("  Port: %d", cfg.Port)

	// Verify and prepare files
	filesMap := make(map[string]model.FileDto)
	pathsMap := make(map[string]string)

	for _, file := range files {
		fileInfo, err := os.Stat(file)
		if err != nil {
			return fmt.Errorf("file not found: %s", file)
		}

		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("failed to open file for detection: %w", err)
		}
		buffer := make([]byte, 512)
		n, _ := f.Read(buffer)
		contentType := http.DetectContentType(buffer[:n])
		f.Close()

		modTime := fileInfo.ModTime().Format(time.RFC3339)
		id := uuid.NewString()

		fileDto := model.FileDto{
			ID:       id,
			FileName: filepath.Base(file),
			Size:     fileInfo.Size(),
			FileType: contentType,
			Metadata: &model.FileMetadata{
				Modified: &modTime,
			},
		}

		filesMap[id] = fileDto
		pathsMap[id] = file
		zap.S().Infof("  Sharing: %s (%s)", filepath.Base(file), cli.FormatBytes(fileInfo.Size()))
	}

	// Create server
	srv := server.NewServer(cfg, zap.S())
	sendService := srv.GetSendService()

	// Register files in session
	_, err := sendService.CreateSession(filesMap, pathsMap)
	if err != nil {
		return fmt.Errorf("failed to create send session: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Initialize discovery service with Download: true
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
		Download:    true,
		Announce:    true,
	}

	multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto, zap.S())
	httpDiscoverer := discovery.NewHTTPDiscovery(nil, cfg.ToRegisterDto(), nil, zap.S())
	multicast.SetHTTPDiscoverer(httpDiscoverer)

	discoverySvc := discovery.NewService(discoverySvcConfig, multicast, zap.S())

	// Start server first
	serverErrChan := make(chan error, 1)
	serverReadyChan := make(chan struct{}, 1)
	go func() {
		serverErrChan <- srv.Start(ctx, serverReadyChan)
	}()

	// Wait for server to be ready
	select {
	case err := <-serverErrChan:
		return fmt.Errorf("server failed: %w", err)
	case <-serverReadyChan:
	}

	// Start discovery AFTER server is ready
	err = discoverySvc.Start(ctx, cfg.Alias, cfg.Port, cfg.SecurityContext.CertificateHash, cfg.DeviceType, cfg.DeviceModel, cfg.HttpsEnabled)
	if err != nil {
		return fmt.Errorf("discovery service failed: %w", err)
	}

	zap.S().Infof("Server ready! Waiting for connections...")
	zap.S().Infof("Press Ctrl+C to stop sharing")

	// Wait for server to finish
	if err := <-serverErrChan; err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	discoverySvc.Stop()
	zap.S().Infof("Web share stopped")
	return nil
}

func (app *Application) runDiscover(cfg *config.Config, timeout *int, jsonOutput *bool, quiet *bool) error {
	// Increase default timeout for better reliability
	discoverTimeout := *timeout
	if discoverTimeout < 10 {
		discoverTimeout = 10
	}

	if !*quiet {
		zap.S().Infof("Discovering devices (timeout: %ds)...", discoverTimeout)
		zap.S().Infof("  Multicast group: %s", cfg.MulticastGroup)
		zap.S().Infof("  Port: %d", cfg.Port)
		zap.S().Infof("  Protocol: %s", func() string {
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

	multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto, zap.S())
	discoverySvc := discovery.NewService(discoverySvcConfig, multicast, zap.S())

	discoverySvc.AddDeviceHandler(func(device *model.Device) {
		if !*quiet {
			zap.S().Infof("Found: %s (%s) [%s] Port: %d", device.Alias, device.IP, device.Protocol, device.Port)
		}
	})

	// Perform discovery
	discoverCtx, cancel := context.WithTimeout(context.Background(), time.Duration(discoverTimeout)*time.Second)
	defer cancel()

	foundDevices, err := discoverySvc.Discover(discoverCtx, cfg.Alias, cfg.Port, cfg.SecurityContext.CertificateHash, cfg.DeviceType, cfg.DeviceModel, cfg.HttpsEnabled, false)
	if err != nil && !*quiet {
		zap.S().Warnf("Discovery completed with warnings: %v", err)
	}

	if !*quiet && len(foundDevices) == 0 {
		zap.S().Warnf("No devices discovered. If you expected to see a device, check:\n- That both devices are on the same Wi-Fi network\n- That firewalls are not blocking UDP port %d\n- That AP/Client Isolation is disabled on your router\n- That the LocalSend app is open and in receive mode", cfg.Port)
	}

	return app.displayDevices(foundDevices, *jsonOutput, *quiet, "multicast discovery")
}

func (app *Application) runScan(cfg *config.Config, timeout *int, port *int, jsonOutput *bool, quiet *bool) error {
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
	localIPs, err := network.GetLocalIPAddresses()
	if err != nil {
		return fmt.Errorf("failed to get local network IPs: %w", err)
	}

	var ips []net.IP
	for _, ip := range localIPs {
		subnetIPs := network.GetSubnetIPs(ip)
		ips = append(ips, subnetIPs...)
	}

	if !*quiet {
		zap.S().Infof("Scanning network on port %d (timeout: %ds)...", scanPort, scanTimeout)
		zap.S().Infof("  Scanning %d IP addresses (derived from %d local interfaces)...", len(ips), len(localIPs))
		zap.S().Infof("  Protocols: HTTPS first, then HTTP fallback")
	}

	// Initialize HTTP discovery
	httpDiscoverer := discovery.NewHTTPDiscovery(nil, cfg.ToRegisterDto(), nil, zap.S())

	// Perform scan
	scanCtx, cancel := context.WithTimeout(context.Background(), time.Duration(scanTimeout)*time.Second)
	defer cancel()

	foundDevices, err := httpDiscoverer.ScanNetwork(scanCtx, ips, scanPort)
	if err != nil && !*quiet {
		zap.S().Warnf("Scan completed with warnings: %v", err)
	}

	if !*quiet && len(foundDevices) == 0 {
		zap.S().Warnf("No devices found during scan. If you expected to see a device, check:\n- That both devices are on the same Wi-Fi network\n- That firewalls are not blocking TCP ports %d (HTTP/HTTPS)\n- That AP/Client Isolation is disabled on your router\n- That the LocalSend app is open and in receive mode", scanPort)
	}

	return app.displayDevices(foundDevices, *jsonOutput, *quiet, "HTTP scan")
}

func (app *Application) runSend(cfg *config.Config, files []string, to *string, port *int, timeout *int, alias *string) error {
	// Validate required parameters
	if len(files) == 0 {
		return fmt.Errorf("file parameter is required (use --file)")
	}
	if *to == "" {
		return fmt.Errorf("target device parameter is required (use --to)")
	}

	// Check if files exist
	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", file)
		}
	}

	// Apply overrides
	if *alias != "" {
		cfg.Alias = *alias
	}

	zap.S().Infof("Sending %d files", len(files))
	for _, file := range files {
		fileInfo, err := os.Stat(file)
		if err == nil {
			zap.S().Infof("  - %s (%s)", filepath.Base(file), cli.FormatBytes(fileInfo.Size()))
		}
	}
	zap.S().Infof("  To: %s", *to)
	zap.S().Infof("  From: %s", cfg.Alias)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// Send files
	err := send.SendFiles(ctx, cfg, files, *to, *port, zap.S())
	if err != nil {
		return fmt.Errorf("failed to send files: %w", err)
	}

	zap.S().Infof("Files sent successfully!")
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
