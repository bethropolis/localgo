package cmd

import (
	"os"
	"path/filepath"
)

// pidFilePath returns the absolute path to the daemon PID file.
func pidFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "localgo", "localgo.pid"), nil
}
