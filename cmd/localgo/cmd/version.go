package cmd

import (
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/spf13/cobra"
)

var (
	Version   = "0.4.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		help.ShowVersion(Version, GitCommit, BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
