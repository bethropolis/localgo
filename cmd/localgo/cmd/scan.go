package cmd

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	scantimeout    int
	scanport       int
	scanjsonOutput bool
	scanquiet      bool
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the network for LocalGo devices using HTTP",
	RunE: func(cmd *cobra.Command, args []string) error {

		// Increase default timeout for better reliability
		scanTimeout := scantimeout
		if scanTimeout < 15 {
			scanTimeout = 15
		}

		scanPort := Cfg.Port
		if scanport > 0 {
			scanPort = scanport
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

		if !scanquiet {
			cli.PrintHeader(fmt.Sprintf("Scanning network on port %d (timeout: %ds)...", scanPort, scanTimeout))
			cli.PrintInfo("Scanning %d IP addresses (derived from %d local interfaces)...", len(ips), len(localIPs))
			cli.PrintInfo("Protocols: HTTPS first, then HTTP fallback")
		}

		// Initialize HTTP discovery
		httpDiscoverer := discovery.NewHTTPDiscovery(nil, Cfg.ToRegisterDto(), nil, zap.S())

		// Perform scan
		scanCtx, cancel := context.WithTimeout(context.Background(), time.Duration(scanTimeout)*time.Second)
		defer cancel()

		foundDevices, err := httpDiscoverer.ScanNetwork(scanCtx, ips, scanPort)
		if err != nil && !scanquiet {
			zap.S().Warnf("Scan completed with warnings: %v", err)
			cli.PrintWarning("Scan completed with warnings: %v", err)
		}

		if !scanquiet && len(foundDevices) == 0 {
			zap.S().Warnf("No devices found during scan")
			cli.PrintWarning("No devices found during scan. Check your firewall or network.")
		}

		return displayDevices(foundDevices, scanjsonOutput, scanquiet, "HTTP scan")
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().IntVar(&scantimeout, "timeout", 15, "Scan timeout in seconds")
	scanCmd.Flags().IntVar(&scanport, "port", 0, "Port to scan")
	scanCmd.Flags().BoolVar(&scanjsonOutput, "json", false, "Output in JSON format")
	scanCmd.Flags().BoolVar(&scanquiet, "quiet", false, "Quiet mode")

	scanCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("scan"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
