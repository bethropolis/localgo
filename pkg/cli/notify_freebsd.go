//go:build freebsd

package cli

import "log"

func notifyPlatform(title, body string) {
	log.Printf("[notification] %s: %s", title, body)
}
