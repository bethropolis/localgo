package cli

import "github.com/acarl005/stripansi"

// Sanitize strips ANSI escape sequences from a string to prevent ANSI injection
// attacks when displaying untrusted data from remote peers.
func Sanitize(s string) string {
	return stripansi.Strip(s)
}
