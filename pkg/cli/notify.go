package cli

import (
	"os"
	"os/exec"
	"strings"

	"github.com/gen2brain/beeep"
)

// notificationCmd holds a user-configured custom notification command.
var notificationCmd string

// SetNotificationCmd sets a custom notification command.
// The command is called with the title and body as the last two arguments.
func SetNotificationCmd(cmd string) {
	notificationCmd = cmd
}

// Notify sends a native desktop notification. Icon is empty (system default).
// No-op in container environments.
func Notify(title, body string) {
	if IsContainer() {
		return
	}
	if notificationCmd != "" {
		parts := strings.Fields(notificationCmd)
		if len(parts) > 0 {
			c := exec.Command(parts[0], append(parts[1:], title, body)...)
			c.Run() // best-effort
		}
		return
	}
	beeep.Notify(title, body, "")
}

// IsContainer returns true if LocalGo is running inside a Docker/Podman container.
func IsContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if os.Getenv("container") != "" {
		return true
	}
	return false
}
