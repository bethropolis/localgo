//go:build !windows && !darwin

package cli

import (
	"log"
	"os/exec"
)

// notifyPlatform dispatches desktop notifications via notify-send.
// Falls back to logging when no notifier is available.
func notifyPlatform(title, body string) {
	if _, err := exec.LookPath("notify-send"); err != nil {
		log.Printf("[notification] %s: %s", title, body)
		return
	}
	cmd := exec.Command("notify-send", "-a", "localgo", title, body)
	cmd.Run()
}
