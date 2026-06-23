package cli

import (
	"os"

	"github.com/gen2brain/beeep"
)

// Notify sends a native desktop notification. Icon is empty (system default).
// No-op in container environments.
func Notify(title, body string) {
	if IsContainer() {
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
