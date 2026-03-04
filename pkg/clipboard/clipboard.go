// Package clipboard provides a thin wrapper around the system clipboard.
// It attempts to write text to the clipboard using golang.design/x/clipboard,
// and returns an error if the clipboard is unavailable (e.g. headless servers,
// Android, or environments without a display server).
package clipboard

import (
	"fmt"

	"golang.design/x/clipboard"
)

// initialized tracks whether clipboard.Init() succeeded.
var initialized bool

func init() {
	if err := clipboard.Init(); err == nil {
		initialized = true
	}
}

// Write copies text to the system clipboard.
// Returns an error when no clipboard is available (headless / no display server).
func Write(text string) error {
	if !initialized {
		return fmt.Errorf("clipboard unavailable: no display server or unsupported platform")
	}
	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}

// Available reports whether the clipboard is usable on this system.
func Available() bool {
	return initialized
}
