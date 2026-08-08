//go:build android

package clipboard

import "os/exec"

func detect() *clipProvider {
	if lookPath("termux-clipboard-set") && lookPath("termux-clipboard-get") {
		return &clipProvider{
			cmd:     "termux-clipboard-set",
			readCmd: "termux-clipboard-get",
		}
	}
	return nil
}

func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
