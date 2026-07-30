package cli

import (
	"os"
	"os/exec"
	"strings"
)

var notificationCmd string

func SetNotificationCmd(cmd string) {
	notificationCmd = cmd
}

func Notify(title, body string) {
	if IsContainer() {
		return
	}
	if notificationCmd != "" {
		parts := strings.Fields(notificationCmd)
		if len(parts) > 0 {
			c := exec.Command(parts[0], append(parts[1:], title, body)...)
			c.Run()
		}
		return
	}
	notifyPlatform(title, body)
}

func IsContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if os.Getenv("container") != "" {
		return true
	}
	return false
}
