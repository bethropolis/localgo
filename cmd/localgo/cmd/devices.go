package cmd

import (
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/spf13/cobra"
)

var devicesjsonOutput bool

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "Show all recently discovered devices on the network",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Just run a quick discovery
		discovertimeout = 2
		discoverjsonOutput = devicesjsonOutput
		discoverquiet = true
		return discoverCmd.RunE(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(devicesCmd)
	devicesCmd.Flags().BoolVar(&devicesjsonOutput, "json", false, "Output in JSON format")

	devicesCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("devices"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
