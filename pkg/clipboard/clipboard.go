// Package clipboard provides a thin wrapper for writing to and reading from the
// system clipboard. It uses platform-native CLI tools (xclip/xsel/wl-copy on Linux,
// pbcopy/pbpaste on macOS, clip.exe/PowerShell on Windows) so that no CGO or
// display-server headers are required at build time. Availability is determined at
// runtime by probing for the tools.
package clipboard

import (
	"fmt"
	"os/exec"
	"strings"
)

// provider holds the resolved clipboard commands for this run.
// nil means no clipboard tool was found.
var provider *clipProvider

type clipProvider struct {
	cmd      string
	args     []string
	readCmd  string
	readArgs []string
}

func init() {
	provider = detect()
}

// Write copies text to the system clipboard.
// Returns an error when no suitable clipboard tool is available.
func Write(text string) error {
	if provider == nil {
		return fmt.Errorf("clipboard unavailable: no supported tool found (install xclip, xsel, wl-copy, pbcopy, or clip.exe)")
	}
	cmd := exec.Command(provider.cmd, provider.args...) //nolint:gosec
	cmd.Stdin = strings.NewReader(text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clipboard write failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Read retrieves text from the system clipboard.
// Returns an error when no suitable clipboard tool is available.
func Read() (string, error) {
	if provider == nil || provider.readCmd == "" {
		return "", fmt.Errorf("clipboard read unavailable: no supported tool found")
	}
	cmd := exec.Command(provider.readCmd, provider.readArgs...) //nolint:gosec
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("clipboard read failed: %w", err)
	}
	// Normalize Windows CRLF line endings to unix LF
	return strings.ReplaceAll(string(out), "\r\n", "\n"), nil
}

// Available reports whether a clipboard tool was found on this system.
func Available() bool {
	return provider != nil
}
