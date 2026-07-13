//go:build linux || freebsd

package clipboard

import (
	"os"
	"os/exec"
)

// detect probes for available clipboard tools on Linux and FreeBSD.
// Prefers Wayland (wl-copy) when WAYLAND_DISPLAY is set, then X11 tools.
func detect() *clipProvider {
	// Wayland — only if WAYLAND_DISPLAY is actually set
	if os.Getenv("WAYLAND_DISPLAY") != "" && lookPath("wl-copy") && lookPath("wl-paste") {
		return &clipProvider{
			cmd:     "wl-copy",
			readCmd: "wl-paste",
		}
	}
	// X11 via xclip
	if lookPath("xclip") {
		return &clipProvider{
			cmd:      "xclip",
			args:     []string{"-selection", "clipboard"},
			readCmd:  "xclip",
			readArgs: []string{"-selection", "clipboard", "-o"},
		}
	}
	// X11 via xsel
	if lookPath("xsel") {
		return &clipProvider{
			cmd:      "xsel",
			args:     []string{"--clipboard", "--input"},
			readCmd:  "xsel",
			readArgs: []string{"--clipboard", "--output"},
		}
	}
	return nil
}

func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
