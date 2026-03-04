//go:build linux

package clipboard

import "os/exec"

// detect probes for available clipboard tools on Linux.
// Prefers Wayland (wl-copy) when WAYLAND_DISPLAY is set, then X11 tools.
func detect() *clipProvider {
	// Wayland
	if lookPath("wl-copy") {
		return &clipProvider{cmd: "wl-copy"}
	}
	// X11 via xclip
	if lookPath("xclip") {
		return &clipProvider{cmd: "xclip", args: []string{"-selection", "clipboard"}}
	}
	// X11 via xsel
	if lookPath("xsel") {
		return &clipProvider{cmd: "xsel", args: []string{"--clipboard", "--input"}}
	}
	// xdotool / xdg-open are not clipboard tools; nothing else to try
	return nil
}

func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
