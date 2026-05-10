package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/model"
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

		// Increase default timeout for better reliability
		discoverTimeout := discovertimeout
		if discoverTimeout < 10 {
			discoverTimeout = 10
		}

		if !discoverquiet {
			zap.S().Infof("Discovering devices (timeout: %ds)...", discoverTimeout)
			zap.S().Infof("  Multicast group: %s", Cfg.MulticastGroup)
			zap.S().Infof("  Port: %d", Cfg.Port)
			zap.S().Infof("  Protocol: %s", func() string {
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
		multicastDto := Cfg.ToMulticastDto(false)

		multicast := discovery.NewMulticastDiscovery(discoverySvcConfig.MulticastConfig, multicastDto, zap.S())
		discoverySvc := discovery.NewService(discoverySvcConfig, multicast, zap.S())

		discoverySvc.AddDeviceHandler(func(device *model.Device) {
			if !discoverquiet {
				zap.S().Infof("Found: %s (%s) [%s] Port: %d", device.Alias, device.IP, device.Protocol, device.Port)
			}
		})

		// Perform discovery
		discoverCtx, cancel := context.WithTimeout(context.Background(), time.Duration(discoverTimeout)*time.Second)
		defer cancel()

		foundDevices, err := discoverySvc.Discover(discoverCtx, Cfg.Alias, Cfg.Port, Cfg.SecurityContext.CertificateHash, Cfg.DeviceType, Cfg.DeviceModel, Cfg.HttpsEnabled, false)
		if err != nil && !discoverquiet {
			zap.S().Warnf("Discovery completed with warnings: %v", err)
		}

		if !discoverquiet && len(foundDevices) == 0 {
			zap.S().Warnf("No devices discovered. If you expected to see a device, check:\n- That both devices are on the same Wi-Fi network\n- That firewalls are not blocking UDP port %d\n- That AP/Client Isolation is disabled on your router\n- That the LocalSend app is open and in receive mode", Cfg.Port)
		}

		return displayDevices(foundDevices, discoverjsonOutput, discoverquiet, "multicast discovery")
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
	discoverCmd.Flags().IntVar(&discovertimeout, "timeout", 5, "Discovery timeout in seconds")
	discoverCmd.Flags().BoolVar(&discoverjsonOutput, "json", false, "Output in JSON format")
	discoverCmd.Flags().BoolVar(&discoverquiet, "quiet", false, "Quiet mode")

	discoverCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("discover"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
