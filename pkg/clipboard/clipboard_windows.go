//go:build windows

package clipboard

import "os/exec"

// detect probes for PowerShell (Set-Clipboard) and clip.exe fallback.
// PowerShell handles Unicode/UTF-8 correctly; clip.exe with stdin piping
// can mangle non-ASCII characters.
func detect() *clipProvider {
	if lookPath("powershell") {
		return &clipProvider{
		cmd:      "powershell",
		args:     []string{"-NoProfile", "-Command", "$input | Set-Clipboard"},
			readCmd:  "powershell",
			readArgs: []string{"-NoProfile", "-Command", "Get-Clipboard"},
		}
	}
	if lookPath("clip") {
		return &clipProvider{
			cmd:     "clip",
			readCmd: "",
		}
	}
	return nil
}

func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
