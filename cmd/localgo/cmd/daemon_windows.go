//go:build windows

package cmd

import "fmt"

func daemonize() error {
	return fmt.Errorf("daemon mode is not supported on Windows; use 'localgo serve' in a terminal or run as a background job")
}
