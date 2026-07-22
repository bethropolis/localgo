//go:build !windows

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func daemonize() error {
	var childArgs []string
	for _, a := range os.Args[1:] {
		if a == "--daemon" || a == "-d" {
			continue
		}
		childArgs = append(childArgs, a)
	}
	child := exec.Command(os.Args[0], childArgs...)
	child.Env = append(os.Environ(), "LOCALGO_DAEMON_CHILD=1")
	child.Stdin = nil
	child.Stdout = nil
	child.Stderr = nil
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := child.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	pidPath, err := pidFilePath()
	if err != nil {
		return fmt.Errorf("cannot determine pid file path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		return fmt.Errorf("cannot create pid directory: %w", err)
	}
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", child.Process.Pid)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	fmt.Printf("LocalGo daemon started (PID %d)\n", child.Process.Pid)
	os.Exit(0)
	return nil
}
