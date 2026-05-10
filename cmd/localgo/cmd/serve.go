package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	serveport        int
	serveuseHTTP     bool
	servepin         string
	servealias       string
	servedir         string
	servequiet       bool
	serveinterval    int
	serveautoAccept  bool
	servenoClipboard bool
	servehistory     string
	serveexecHook    string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the LocalGo server to receive files",
	RunE: func(cmd *cobra.Command, args []string) error {

		// Apply overrides
		if serveport > 0 {
			Cfg.Port = serveport
		}
		if serveuseHTTP {
			Cfg.HttpsEnabled = false
		}
		if servepin != "" {
			Cfg.PIN = servepin
		}
		if servealias != "" {
			Cfg.Alias = servealias
		}
		if servedir != "" {
			Cfg.DownloadDir = servedir
		}
		if serveautoAccept {
			Cfg.AutoAccept = true
		}
		if servenoClipboard {
			Cfg.NoClipboard = true
		}
		if servehistory != "" {
			Cfg.HistoryFile = servehistory
		}
		if serveexecHook != "" {
			Cfg.ExecHook = serveexecHook
		}
		if servequiet {
			Cfg.Quiet = true
		}

		// Create download directory if it doesn't exist
		if err := os.MkdirAll(Cfg.DownloadDir, 0755); err != nil {
			return fmt.Errorf("failed to create download directory: %w", err)
		}

		protocol := "HTTPS"
		if !Cfg.HttpsEnabled {
			protocol = "HTTP"
		}

		logFn := zap.S().Infof
		if servequiet {
			logFn = zap.S().Warnf
		}

		logFn("Starting LocalGo server")
		logFn("  Alias: %s", Cfg.Alias)
		logFn("  Protocol: %s", protocol)
		logFn("  Port: %d", Cfg.Port)
		logFn("  Download Directory: %s", Cfg.DownloadDir)
		if Cfg.PIN != "" {
			logFn("  PIN Protection: Enabled")
		}
		logFn("  Fingerprint: %s", Cfg.SecurityContext.CertificateHash[:16]+"...")

		// Context for graceful shutdown
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		// Initialize discovery service
		discoverySvcConfig := discovery.DefaultServiceConfig()
		discoverySvcConfig.MulticastConfig.Port = Cfg.Port
		discoverySvcConfig.MulticastConfig.MulticastAddr = fmt.Sprintf("%s:%d", Cfg.MulticastGroup, Cfg.Port)

		if serveinterval > 0 {
			discoverySvcConfig.AnnounceInterval = time.Duration(serveinterval) * time.Second
		}
		multicastDto := Cfg.ToMulticastDto(false)

		multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto, zap.S())

		// Create HTTPDiscoverer for backchannel (HTTP response to multicast)
		httpDiscoverer := discovery.NewHTTPDiscovery(nil, Cfg.ToRegisterDto(), nil, zap.S())
		multicast.SetHTTPDiscoverer(httpDiscoverer)

		discoverySvc := discovery.NewService(discoverySvcConfig, multicast, zap.S())

		discoverySvc.AddDeviceHandler(func(device *model.Device) {
			if !servequiet {
				zap.S().Infof("Device discovered: %s (%s)", device.Alias, device.IP)
			}
		})

		// Start server first
		srv := server.NewServer(Cfg, zap.S())

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
		err := discoverySvc.Start(ctx, Cfg.Alias, Cfg.Port, Cfg.SecurityContext.CertificateHash, Cfg.DeviceType, Cfg.DeviceModel, Cfg.HttpsEnabled)
		if err != nil {
			return fmt.Errorf("discovery service failed: %w", err)
		}

		if !servequiet {
			zap.S().Infof("Server ready! Waiting for files...")
			zap.S().Infof("Press Ctrl+C to stop")
		}

		// Wait for server to finish
		if err := <-serverErrChan; err != nil {
			return fmt.Errorf("server failed: %w", err)
		}

		discoverySvc.Stop()
		if servequiet {
			zap.S().Warnf("Server stopped")
		} else {
			zap.S().Infof("Server stopped")
		}
		return nil
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVar(&serveport, "port", 0, "Port to run the server on (default: from config)")
	serveCmd.Flags().BoolVar(&serveuseHTTP, "http", false, "Use HTTP instead of HTTPS")
	serveCmd.Flags().StringVar(&servepin, "pin", "", "PIN for authentication")
	serveCmd.Flags().StringVar(&servealias, "alias", "", "Device alias (default: from config)")
	serveCmd.Flags().StringVar(&servedir, "dir", "", "Download directory (default: from config)")
	serveCmd.Flags().BoolVar(&servequiet, "quiet", false, "Quiet mode - minimal output")
	serveCmd.Flags().IntVar(&serveinterval, "interval", 30, "Discovery announcement interval in seconds")
	serveCmd.Flags().BoolVar(&serveautoAccept, "auto-accept", false, "Auto-accept incoming files without prompting")
	serveCmd.Flags().BoolVar(&servenoClipboard, "no-clipboard", false, "Save incoming text as a file instead of copying to clipboard")
	serveCmd.Flags().StringVar(&servehistory, "history", "", "Path to transfer history JSONL file (default: ~/.local/share/localgo/history.jsonl)")
	serveCmd.Flags().StringVar(&serveexecHook, "exec", "", "Shell command to run after each received file")

	serveCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("serve"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
