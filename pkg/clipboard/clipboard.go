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
		return fmt.Errorf("clipboard write failed (%s): %w: %s", provider.cmd, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Read retrieves text from the system clipboard.
// Returns an error when no suitable clipboard tool is available.
func Read() (string, error) {
	if provider == nil || provider.readCmd == "" {
		return "", fmt.Errorf("clipboard read unavailable: no supported tool found (install xclip, xsel, wl-paste, pbpaste, or Get-Clipboard)")
	}
	cmd := exec.Command(provider.readCmd, provider.readArgs...) //nolint:gosec
	out, err := cmd.Output()
	if err != nil {
		// Some tools (xclip, wl-paste) exit with 1 when the clipboard is empty
		// and produce no output. Treat this as empty, not an error.
		if len(out) == 0 {
			return "", nil
		}
		return "", fmt.Errorf("clipboard read failed (%s): %w", provider.readCmd, err)
	}
	// Normalize Windows CRLF line endings to unix LF
	return strings.ReplaceAll(string(out), "\r\n", "\n"), nil
}

// Available reports whether a clipboard tool was found on this system.
func Available() bool {
	return provider != nil
}

// OverrideProvider replaces the auto-detected clipboard tool with custom commands.
// Empty strings are ignored (auto-detected tool kept for that direction, if any).
// Returns an error if the command string is non-empty but yields no tokens.
func OverrideProvider(writeCmd, readCmd string) {
	if writeCmd == "" && readCmd == "" {
		return
	}
	p := &clipProvider{}
	if writeCmd != "" {
		if wp := strings.Fields(writeCmd); len(wp) > 0 {
			p.cmd = wp[0]
			p.args = wp[1:]
		}
	}
	if p.cmd == "" && provider != nil {
		p.cmd = provider.cmd
		p.args = provider.args
	}
	if readCmd != "" {
		if rp := strings.Fields(readCmd); len(rp) > 0 {
			p.readCmd = rp[0]
			p.readArgs = rp[1:]
		}
	}
	if p.readCmd == "" && provider != nil {
		p.readCmd = provider.readCmd
		p.readArgs = provider.readArgs
	}
	if p.cmd != "" || p.readCmd != "" {
		provider = p
	}
}
