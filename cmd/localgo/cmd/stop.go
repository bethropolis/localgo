package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/help"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running LocalGo daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		pidPath, err := pidFilePath()
		if err != nil {
			return fmt.Errorf("cannot determine pid file path: %w", err)
		}

		data, err := os.ReadFile(pidPath)
		if err != nil {
			if os.IsNotExist(err) {
				cli.PrintWarning("No running LocalGo daemon found (PID file not found)")
				return nil
			}
			return fmt.Errorf("failed to read PID file %s: %w", pidPath, err)
		}

		pidStr := strings.TrimSpace(string(data))
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			return fmt.Errorf("invalid PID in %s: %q", pidPath, pidStr)
		}

		return stopDaemonProcess(pid, pidPath)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("stop"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
