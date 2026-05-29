package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/discovery"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	devicesjsonOutput bool
	devicesProbe      bool
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "Show recently discovered devices on the network",
	RunE: func(cmd *cobra.Command, args []string) error {
		peerCache := discovery.NewPeerCache(nil)
		peers := peerCache.GetPeers()

		if len(peers) == 0 {
			cli.PrintInfo("No devices in local cache. Run 'localgo discover' or 'localgo scan' to find devices.")
			return nil
		}

		if devicesProbe {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_ = spinner.New().
				Title("Probing cached devices...").
				Action(func() {
					discovery.ProbeCached(ctx, peerCache, func(d *model.Device) {}, nil)
				}).
				Run()

			peers = peerCache.GetPeers()
		}

		if devicesjsonOutput {
			return displayDevices(peers, true, false, "cache")
		}

		if Cfg != nil && Cfg.Private {
			peers = anonymizeDeviceSlice(peers)
		}

		for i := 0; i < len(peers); i++ {
			for j := i + 1; j < len(peers); j++ {
				if peers[i].LastSeen.Before(peers[j].LastSeen) {
					peers[i], peers[j] = peers[j], peers[i]
				}
			}
		}

		titleStyle := cli.HeaderStyle.Padding(0, 1).MarginBottom(1)
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
		rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))

		fmt.Println(titleStyle.Render(cli.IconDevice+"  Recently Discovered Devices") + "\n")

		colWidths := []int{18, 16, 10, 12, 16}

		fmt.Printf("%s  %s  %s  %s  %s\n",
			padRight(headerStyle.Render("ALIAS"), colWidths[0]),
			padRight(headerStyle.Render("IP ADDRESS"), colWidths[1]),
			padRight(headerStyle.Render("PORT"), colWidths[2]),
			padRight(headerStyle.Render("TYPE"), colWidths[3]),
			padRight(headerStyle.Render("LAST SEEN"), colWidths[4]),
		)
		fmt.Println(mutedStyle.Render(strings.Repeat("-", 80)))

		now := time.Now()
		for _, d := range peers {
			lastSeenStr := "Unknown"
			if !d.LastSeen.IsZero() {
				diff := now.Sub(d.LastSeen)
				if diff < 1*time.Minute {
					lastSeenStr = cli.SuccessStyle.Render("Online")
				} else if diff < 1*time.Hour {
					lastSeenStr = fmt.Sprintf("%dm ago", int(diff.Minutes()))
				} else if diff < 24*time.Hour {
					lastSeenStr = fmt.Sprintf("%dh ago", int(diff.Hours()))
				} else {
					lastSeenStr = fmt.Sprintf("%dd ago", int(diff.Hours()/24))
				}
			}

			deviceTypeStr := string(d.DeviceType)
			if len(deviceTypeStr) > 0 {
				deviceTypeStr = strings.ToUpper(deviceTypeStr[:1]) + deviceTypeStr[1:]
			}

			fmt.Printf("%s  %s  %s  %s  %s\n",
				padRight(rowStyle.Render(cli.TruncateString(d.Alias, 16)), colWidths[0]),
				padRight(rowStyle.Render(d.IP), colWidths[1]),
				padRight(rowStyle.Render(fmt.Sprintf("%d", d.Port)), colWidths[2]),
				padRight(rowStyle.Render(deviceTypeStr), colWidths[3]),
				padRight(lastSeenStr, colWidths[4]),
			)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(devicesCmd)
	devicesCmd.Flags().BoolVar(&devicesjsonOutput, "json", false, "Output in JSON format")
	devicesCmd.Flags().BoolVarP(&devicesProbe, "probe", "p", false, "Probe cached devices to verify if they are currently online")

	devicesCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("devices"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
