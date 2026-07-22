package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

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

		process, err := os.FindProcess(pid)
		if err != nil {
			os.Remove(pidPath)
			cli.PrintWarning("No running LocalGo daemon found (process %d not found)", pid)
			return nil
		}

		// Check if process is alive (Signal(0) is a liveness probe)
		if err := process.Signal(syscall.Signal(0)); err != nil {
			os.Remove(pidPath)
			cli.PrintWarning("No running LocalGo daemon found (process %d is dead)", pid)
			return nil
		}

		cli.PrintInfo("Stopping LocalGo daemon (PID %d)...", pid)
		process.Signal(syscall.SIGTERM)

		// Poll for exit up to 5 seconds
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			if err := process.Signal(syscall.Signal(0)); err != nil {
				os.Remove(pidPath)
				cli.PrintSuccess("LocalGo daemon stopped")
				return nil
			}
		}

		// Timeout — force kill
		cli.PrintWarning("Daemon did not stop gracefully, sending SIGKILL...")
		process.Kill()
		os.Remove(pidPath)
		cli.PrintSuccess("LocalGo daemon killed")
		return nil
	},
}

func pidFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "localgo", "localgo.pid"), nil
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if h := help.GetCommandHelp("stop"); h != nil {
			help.ShowCommandHelp(*h)
		}
	})
}
