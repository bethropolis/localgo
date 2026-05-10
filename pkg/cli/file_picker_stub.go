//go:build !((linux || darwin || windows) && cgo)

package cli

import "fmt"

func pickFilesNative() (string, error) {
	return "", fmt.Errorf("native file picker requires CGO")
}