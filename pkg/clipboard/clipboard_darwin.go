//go:build darwin

package clipboard

import "os/exec"

// detect probes for pbcopy, which ships with every macOS installation.
func detect() *clipProvider {
	if lookPath("pbcopy") {
		return &clipProvider{cmd: "pbcopy"}
	}
	return nil
}

func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
