//go:build windows

package clipboard

import "os/exec"

// detect probes for clip.exe, which ships with every Windows installation.
func detect() *clipProvider {
	if lookPath("clip") {
		return &clipProvider{cmd: "clip"}
	}
	return nil
}

func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
