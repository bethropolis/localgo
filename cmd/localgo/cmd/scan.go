package cmd

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/network"
	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	scanrange      string
	scantimeout    int
	scanport       int
	scanjsonOutput bool
	scanquiet      bool
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the network for LocalGo devices using HTTP",
	RunE: func(cmd *cobra.Command, args []string) error {

		scanPort := Cfg.Port
		if scanport > 0 {
			scanPort = scanport
		}

		var ips []net.IP

		if scanrange != "" {
			parsedIPs, err := network.ParseCIDRRange(scanrange)
			if err != nil {
				return fmt.Errorf("invalid --range CIDR: %w", err)
			}
			ips = parsedIPs
			if !scanquiet {
				cli.PrintHeader(fmt.Sprintf("Scanning CIDR range %s on port %d (timeout: %ds)...", scanrange, scanPort, scantimeout))
				cli.PrintInfo("Scanning %d IP addresses...", len(ips))
				cli.PrintInfo("Protocols: HTTPS first, then HTTP fallback")
			}
		} else {
			// Get local IPs
			localIPs, err := network.GetLocalIPAddresses()
			if err != nil {
				return fmt.Errorf("failed to get local network IPs: %w", err)
			}

			// Prioritize the subnet connected to the default gateway
			if gwIP, err := network.PrimaryLANIP(); err == nil {
				for i, ip := range localIPs {
					if ip.Equal(gwIP) && i > 0 {
						localIPs[0], localIPs[i] = localIPs[i], localIPs[0]
						break
					}
				}
			}

			for _, ip := range localIPs {
				subnetIPs := network.GetSubnetIPs(ip)
				ips = append(ips, subnetIPs...)
			}

			if !scanquiet {
				cli.PrintHeader(fmt.Sprintf("Scanning network on port %d (timeout: %ds)...", scanPort, scantimeout))
				cli.PrintInfo("Scanning %d IP addresses (derived from %d local interfaces)...", len(ips), len(localIPs))
				cli.PrintInfo("Protocols: HTTPS first, then HTTP fallback")
			}
		}

		// Initialize HTTP discovery
		httpDiscoverer := discovery.NewHTTPDiscovery(nil, Cfg.ToRegisterDto(), nil, zap.S())

		// Perform scan
		scanCtx, cancel := context.WithTimeout(context.Background(), time.Duration(scantimeout)*time.Second)
		defer cancel()

		var foundDevices []*model.Device
		var scanErr error

		if !scanquiet {
			_ = spinner.New().
				Title(fmt.Sprintf("Scanning %d IP addresses on port %d...", len(ips), scanPort)).
				Action(func() {
					foundDevices, scanErr = httpDiscoverer.ScanNetwork(scanCtx, ips, scanPort)
				}).
				Run()
		} else {
			foundDevices, scanErr = httpDiscoverer.ScanNetwork(scanCtx, ips, scanPort)
		}

		if scanErr != nil && !scanquiet {
			zap.S().Warnf("Scan completed with warnings: %v", scanErr)
			cli.PrintWarning("Scan completed with warnings: %v", scanErr)
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
	scanCmd.Flags().StringVar(&scanrange, "range", "", "CIDR range to scan (e.g. 192.168.1.0/24)")
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
