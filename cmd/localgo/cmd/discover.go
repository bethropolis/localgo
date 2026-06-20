package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/charmbracelet/huh/spinner"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	discovertimeout    int
	discoverjsonOutput bool
	discoverquiet      bool
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover LocalGo devices on the network using multicast",
	RunE: func(cmd *cobra.Command, args []string) error {

		if !discoverquiet {
			cli.PrintHeader("Discovering devices")
			cli.PrintInfo("Timeout: %ds", discovertimeout)
			cli.PrintInfo("Multicast group: %s", Cfg.MulticastGroup)
			cli.PrintInfo("Port: %d", Cfg.Port)
			cli.PrintInfo("Protocol: %s", func() string {
				if Cfg.HttpsEnabled {
					return "HTTPS"
				}
				return "HTTP"
			}())
		}

		// Initialize discovery service
		discoverySvcConfig := discovery.DefaultServiceConfig()
		discoverySvcConfig.MulticastConfig.Port = Cfg.Port
		discoverySvcConfig.MulticastConfig.MulticastAddr = fmt.Sprintf("%s:%d", Cfg.MulticastGroup, Cfg.Port)
		discoverySvcConfig.MulticastConfig.InterfaceName = Cfg.MulticastInterface
		multicastDto := Cfg.ToMulticastDto(false)

		multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto, zap.S())

		peerCache := discovery.NewPeerCache(zap.S())
		multicast.SetPeerCache(peerCache)

		discoverySvc := discovery.NewService(discoverySvcConfig, multicast, zap.S())
		discoverySvc.SetPeerCache(peerCache)

		discoverySvc.AddDeviceHandler(func(device *model.Device) {
			if !discoverquiet {
				alias := device.Alias
				if Cfg.Private {
					alias = cli.AnonymizedAlias(device)
				}
				zap.S().Infof("Found: %s (%s) [%s] Port: %d", alias, device.IP, device.Protocol, device.Port)
				cli.PrintSuccess("Found: %s (%s) [%s] Port: %d", alias, device.IP, device.Protocol, device.Port)
			}
		})

		// Perform discovery
		discoverCtx, cancel := context.WithTimeout(context.Background(), time.Duration(discovertimeout)*time.Second)
		defer cancel()

		var foundDevices []*model.Device
		var discErr error

		if !discoverquiet {
			_ = spinner.New().
				Title(fmt.Sprintf("Searching for devices on multicast group %s...", Cfg.MulticastGroup)).
				Action(func() {
			foundDevices, discErr = discoverySvc.Discover(discoverCtx, Cfg.ToMulticastDto(false))
				}).
				Run()
		} else {
			foundDevices, discErr = discoverySvc.Discover(discoverCtx, Cfg.ToMulticastDto(false))
		}

		if discErr != nil && !discoverquiet {
			zap.S().Warnf("Discovery completed with warnings: %v", discErr)
			cli.PrintWarning("Discovery completed with warnings: %v", discErr)
		}

		if !discoverquiet && len(foundDevices) == 0 {
			zap.S().Warnf("No devices discovered")
			cli.PrintWarning("No devices discovered. Check your firewall or network.")
		}

		return displayDevices(foundDevices, discoverjsonOutput, discoverquiet, "multicast discovery")
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
	discoverCmd.Flags().IntVar(&discovertimeout, "timeout", 10, "Discovery timeout in seconds")
	discoverCmd.Flags().BoolVar(&discoverjsonOutput, "json", false, "Output in JSON format")
	discoverCmd.Flags().BoolVar(&discoverquiet, "quiet", false, "Quiet mode")

	discoverCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("discover"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
