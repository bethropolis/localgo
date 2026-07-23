//go:build windows

package cmd

import (
	"os"

	"github.com/bethropolis/localgo/pkg/cli"
)

func stopDaemonProcess(pid int, pidPath string) error {
	cli.PrintInfo("Stopping LocalGo daemon (PID %d)...", pid)

	// On Windows, os.FindProcess always returns a handle even for dead PIDs,
	// so we skip the liveness probe and go straight to Kill.
	process, _ := os.FindProcess(pid)
	if err := process.Kill(); err != nil {
		cli.PrintWarning("No running LocalGo daemon found with PID %d", pid)
	} else {
		cli.PrintSuccess("LocalGo daemon stopped")
	}
	os.Remove(pidPath)
	return nil
}
