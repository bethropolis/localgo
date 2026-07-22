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

		cli.PrintInfo("Stopping LocalGo daemon (PID %d)...", pid)
		if err := process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to signal process %d: %w", pid, err)
		}

		// Wait up to 5 seconds for graceful shutdown
		done := make(chan struct{})
		go func() {
			process.Wait()
			close(done)
		}()

		select {
		case <-done:
			cli.PrintSuccess("LocalGo daemon stopped")
		case <-time.After(5 * time.Second):
			cli.PrintWarning("Daemon did not stop gracefully, sending SIGKILL...")
			process.Kill()
			<-done
			cli.PrintSuccess("LocalGo daemon killed")
		}

		os.Remove(pidPath)
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
