package cmd

import (
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func SetVersionInfo(version, commit, date string) {
	Version = version
	GitCommit = commit
	BuildDate = date
}

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
