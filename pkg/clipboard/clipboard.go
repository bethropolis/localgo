// Package clipboard provides a thin wrapper for writing text to the system clipboard.
// It uses platform-native CLI tools (xclip/xsel on Linux, pbcopy on macOS,
// clip.exe on Windows) so that no CGO or display-server headers are required at
// build time. Availability is determined at runtime by probing for the tools.
package clipboard

import (
	"fmt"
	"os/exec"
	"strings"
)

// provider holds the resolved clipboard write command for this run.
// nil means no clipboard tool was found.
var provider *clipProvider

type clipProvider struct {
	cmd  string
	args []string
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

// Available reports whether a clipboard tool was found on this system.
func Available() bool {
	return provider != nil
}
