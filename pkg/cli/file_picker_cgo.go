//go:build (linux || darwin || windows) && cgo

package cli

import (
	"fmt"

	"github.com/sqweek/dialog"
)

func pickFilesNative() (string, error) {
	path, err := dialog.File().Load()
	if err != nil {
		return "", fmt.Errorf("file picker canceled: %w", err)
	}
	return path, nil
}