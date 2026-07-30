//go:build !freebsd

package cli

import "github.com/gen2brain/beeep"

func notifyPlatform(title, body string) {
	beeep.Notify(title, body, "")
}
