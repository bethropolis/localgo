//go:build windows

package cmd

import (
	"os"

	"github.com/bethropolis/localgo/pkg/cli"
)

func stopDaemonProcess(pid int, pidPath string) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(pidPath)
		cli.PrintWarning("No running LocalGo daemon found (process %d not found)", pid)
		return nil
	}

	cli.PrintInfo("Stopping LocalGo daemon (PID %d)...", pid)
	if err := process.Kill(); err != nil {
		cli.PrintWarning("Failed to kill daemon process (PID %d): %v", pid, err)
	} else {
		cli.PrintSuccess("LocalGo daemon stopped")
	}
	os.Remove(pidPath)
	return nil
}
