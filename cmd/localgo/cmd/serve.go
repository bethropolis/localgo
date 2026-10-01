package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/ipc"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/bethropolis/localgo/pkg/server"
	"github.com/bethropolis/localgo/pkg/server/handlers"
	"github.com/spf13/cobra"
	"github.com/bethropolis/localgo/pkg/logging"
)

var (
	serveport        int
	serveuseHTTP     bool
	servepin         string
	servealias       string
	servedir         string
	servequiet       bool
	servedaemon      bool
	serveinterval    int
	serveautoAccept  bool
	servenoClipboard bool
	servehistory     string
	serveexecHook    string
	serveopen            bool
	servemulticastiface  string
	serveipc             bool
)

// ipcController backs the pkg/ipc control socket with live serve state.
type ipcController struct {
	pending *handlers.PendingRegistry
	peers   *discovery.PeerCache
}

func (c ipcController) Status() ipc.Status {
	protocol := "https"
	if !Cfg.HttpsEnabled {
		protocol = "http"
	}
	fingerprint := ""
	if Cfg.SecurityContext != nil {
		fingerprint = Cfg.SecurityContext.CertificateHash
	}
	pending := 0
	if c.pending != nil {
		pending = len(c.pending.List())
	}
	return ipc.Status{
		Alias:        Cfg.Alias,
		Fingerprint:  fingerprint,
		Port:         Cfg.Port,
		Protocol:     protocol,
		Version:      Version,
		DownloadDir:  Cfg.DownloadDir,
		AutoAccept:   Cfg.AutoAccept,
		PendingCount: pending,
	}
}

func (c ipcController) Devices() []*model.Device {
	if c.peers == nil {
		return nil
	}
	return c.peers.GetPeers()
}

func (c ipcController) Pending() []handlers.PendingTransfer {
	if c.pending == nil {
		return nil
	}
	return c.pending.List()
}

func (c ipcController) DecidePending(pendingID string, accept bool) bool {
	if c.pending == nil {
		return false
	}
	return c.pending.Decide(pendingID, accept)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the LocalGo server to receive files",
	RunE: func(cmd *cobra.Command, args []string) error {

		// Daemon mode: fork into background
		if servedaemon && os.Getenv("LOCALGO_DAEMON_CHILD") != "1" {
			return daemonize()
		}

		// Daemon child: ensure PID file is cleaned up when server exits
		if os.Getenv("LOCALGO_DAEMON_CHILD") == "1" {
			defer func() {
				if pidPath, err := pidFilePath(); err == nil {
					_ = os.Remove(pidPath)
				}
			}()
		}

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
		// Daemon child has no terminal — force auto-accept and quiet
		if os.Getenv("LOCALGO_DAEMON_CHILD") != "" {
			Cfg.AutoAccept = true
			Cfg.Quiet = true
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
		if serveopen {
			Cfg.OpenDir = true
		}
		if servequiet {
			Cfg.Quiet = true
		}
		if servemulticastiface != "" {
			Cfg.MulticastInterface = servemulticastiface
		}

		// Create download directory if it doesn't exist
		if err := os.MkdirAll(Cfg.DownloadDir, 0755); err != nil {
			return fmt.Errorf("failed to create download directory: %w", err)
		}

		protocol := "HTTPS"
		if !Cfg.HttpsEnabled {
			protocol = "HTTP"
		}

		displayAlias := Cfg.Alias
		if Cfg.Private {
			displayAlias = "Anonymous"
		}

		logging.Global().Infof("Starting LocalGo server")
		logging.Global().Infof("Alias: %s", displayAlias)
		logging.Global().Infof("Protocol: %s", protocol)

		if !servequiet {
			cli.PrintHeader("Starting LocalGo server")
			cli.PrintInfo("Alias: %s", displayAlias)
			cli.PrintInfo("Protocol: %s", protocol)
			cli.PrintInfo("Port: %d", Cfg.Port)
			cli.PrintInfo("Download Directory: %s", Cfg.DownloadDir)
			if Cfg.PIN != "" {
				cli.PrintInfo("PIN Protection: Enabled")
			}
			cli.PrintInfo("Fingerprint: %s", Cfg.SecurityContext.CertificateHash[:16]+"...")
		}

		// Context for graceful shutdown
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		// Start server first to determine the actual port
		srv := server.NewServer(Cfg, logging.Global())
		if serveipc {
			srv.SetTransferHookEnabled(true)
		}

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

		// Initialize discovery service AFTER server is ready (Cfg.Port may have
		// changed if the configured port was busy)
		discoverySvcConfig := discovery.DefaultServiceConfig()
		discoverySvcConfig.MulticastConfig.Port = Cfg.Port
		discoverySvcConfig.MulticastConfig.MulticastAddr = fmt.Sprintf("%s:%d", Cfg.MulticastGroup, Cfg.Port)
		discoverySvcConfig.MulticastConfig.InterfaceName = Cfg.MulticastInterface

		if serveinterval > 0 {
			discoverySvcConfig.AnnounceInterval = time.Duration(serveinterval) * time.Second
		}

		multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, Cfg.ToMulticastDto(false), logging.Global())

		// Create HTTPDiscoverer for backchannel (HTTP response to multicast)
		httpDiscoverer := discovery.NewHTTPDiscovery(nil, Cfg.ToRegisterDto(), nil, logging.Global())
		multicast.SetHTTPDiscoverer(httpDiscoverer)

		peerCache := discovery.NewPeerCache(logging.Global())
		multicast.SetPeerCache(peerCache)

		discoverySvc := discovery.NewService(discoverySvcConfig, multicast, logging.Global())
		discoverySvc.SetPeerCache(peerCache)

		// Local control socket for third-party integrations (GUIs, trays,
		// scripts). Transfer approvals route to IPC controllers instead of
		// the interactive prompt while the hook is enabled.
		var ipcSrv *ipc.IPCServer
		if serveipc {
			reg := srv.PendingRegistry()
			if reg == nil {
				return fmt.Errorf("transfer hook not initialized")
			}
			var err error
			ipcSrv, err = ipc.StartIPCServer(ipcController{pending: reg, peers: peerCache})
			if err != nil {
				return fmt.Errorf("failed to start IPC control socket: %w", err)
			}
			defer ipcSrv.Close()
			logging.Global().Infof("IPC control socket: %s", ipcSrv.SocketPath())
			if !servequiet {
				cli.PrintInfo("IPC control socket: %s", ipcSrv.SocketPath())
			}
		}

		discoverySvc.AddDeviceHandler(func(device *model.Device) {
			if !servequiet {
				alias := device.Alias
				if Cfg.Private {
					alias = cli.AnonymizedAlias(device)
				}
				logging.Global().Infof("Device discovered: %s (%s)", alias, device.IP)
				cli.PrintSuccess("Device discovered: %s (%s)", alias, device.IP)
			}
		})

		// Start discovery. On Android/Termux a failure here is NOT fatal: the
		// file server must keep running even when discovery is unavailable
		// (the sandbox blocks the netlink calls discovery relies on).
		// Everywhere else discovery is required, so fail fast.
		if err := discoverySvc.Start(ctx, Cfg.ToMulticastDto(false)); err != nil {
			if runtime.GOOS != "android" {
				return fmt.Errorf("discovery service failed: %w", err)
			}
			logging.Global().Warnf("Discovery unavailable: %v", err)
			if !servequiet {
				cli.PrintWarning("Network discovery unavailable: %v", err)
				cli.PrintWarning("Running without discovery. Use a manual IP (localgo send --ip) or the peer cache.")
			}
		}

		if !servequiet {
			logging.Global().Infof("Server ready! Waiting for files...")
			cli.PrintSuccess("Server ready! Waiting for files...")

			localIPs, err := network.GetLocalIPAddresses()
			if err == nil && len(localIPs) > 0 {
				cli.PrintHeader("\nListening Addresses:")
				for _, ip := range localIPs {
					scheme := "https"
					if !Cfg.HttpsEnabled {
						scheme = "http"
					}
					cli.PrintInfo("  %s://%s:%d", scheme, ip.String(), Cfg.Port)
				}
				fmt.Println()
			}

			cli.PrintWarning("Press Ctrl+C to stop")
		}

		// Wait for server to finish
		if err := <-serverErrChan; err != nil {
			return fmt.Errorf("server failed: %w", err)
		}

		discoverySvc.Stop()
		if servequiet {
			logging.Global().Infof("Server stopped")
		} else {
			logging.Global().Infof("Server stopped")
			cli.PrintInfo("Server stopped")
		}
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
	serveCmd.Flags().BoolVarP(&servedaemon, "daemon", "d", false, "Run server as a background daemon")
	serveCmd.Flags().IntVar(&serveinterval, "interval", 30, "Discovery announcement interval in seconds")
	serveCmd.Flags().BoolVar(&serveautoAccept, "auto-accept", false, "Auto-accept incoming files without prompting")
	serveCmd.Flags().BoolVar(&servenoClipboard, "no-clipboard", false, "Save incoming text as a file instead of copying to clipboard")
	serveCmd.Flags().StringVar(&servehistory, "history", "", "Path to transfer history JSONL file (default: ~/.local/share/localgo/history.jsonl)")
	serveCmd.Flags().StringVar(&serveexecHook, "exec", "", "Shell command to run after each received file")
	serveCmd.Flags().BoolVar(&serveopen, "open", false, "Open download directory after transfer completes")
	serveCmd.Flags().StringVar(&servemulticastiface, "iface", "", "Multicast network interface name")
	serveCmd.Flags().BoolVar(&serveipc, "ipc", false, "Local control socket for third-party apps (status, peers, approve/reject transfers)")

	serveCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("serve"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
