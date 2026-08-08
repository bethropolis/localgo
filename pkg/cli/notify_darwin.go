//go:build darwin

package cli

import (
	"log"
	"os/exec"
	"strings"
)

// notifyPlatform dispatches notifications via osascript.
func notifyPlatform(title, body string) {
	if _, err := exec.LookPath("osascript"); err != nil {
		log.Printf("[notification] %s: %s", title, body)
		return
	}
	script := "display notification " + shellQuote(body) + " with title " + shellQuote(title)
	cmd := exec.Command("osascript", "-e", script)
	cmd.Run()
}

func shellQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
