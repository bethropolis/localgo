//go:build !windows

package cmd

import (
	"os"
	"syscall"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
)

func stopDaemonProcess(pid int, pidPath string) error {
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
	_ = process.Signal(syscall.SIGTERM)

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
	_ = process.Kill()
	os.Remove(pidPath)
	cli.PrintSuccess("LocalGo daemon killed")
	return nil
}
