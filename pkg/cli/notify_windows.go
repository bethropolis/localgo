//go:build windows

package cli

import (
	"log"
	"os/exec"
	"strings"
)

// notifyPlatform dispatches notifications via a PowerShell balloon tip.
func notifyPlatform(title, body string) {
	script := `[reflection.assembly]::loadwithpartialname('System.Windows.Forms')|Out-Null;`
	script += `$n=New-Object Windows.Forms.NotifyIcon;`
	script += `$n.Icon=[Drawing.SystemIcons]::Information;`
	script += `$n.BalloonTipTitle='` + escapePS(title) + `';`
	script += `$n.BalloonTipText='` + escapePS(body) + `';`
	script += `$n.Visible=$true;`
	script += `$n.ShowBalloonTip(5000)`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	if err := cmd.Run(); err != nil {
		log.Printf("[notification] %s: %s", title, body)
	}
}

func escapePS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `'`, `''`)
}
