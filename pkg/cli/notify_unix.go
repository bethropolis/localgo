//go:build !windows && !darwin

package cli

import (
	"log"
	"os/exec"
	"runtime"
)

// notifyPlatform dispatches notifications. On Android/Termux it prefers
// termux-notification; otherwise it uses notify-send. Falls back to logging
// when no notifier is available.
func notifyPlatform(title, body string) {
	// termux-* tools only exist on Android; skip the PATH scan elsewhere.
	if runtime.GOOS == "android" {
		if _, err := exec.LookPath("termux-notification"); err == nil {
			cmd := exec.Command("termux-notification", "--title", title, "--content", body)
			cmd.Run()
			return
		}
	}
	if _, err := exec.LookPath("notify-send"); err != nil {
		log.Printf("[notification] %s: %s", title, body)
		return
	}
	cmd := exec.Command("notify-send", "-a", "localgo", title, body)
	cmd.Run()
}
