//go:build !linux && !android

package network

import "errors"

// ioctlSystemInterfaces is unavailable outside Linux/Android. Those platforms
// rely on net.Interfaces(), which only fails on genuine errors, so no fallback
// is needed.
func ioctlSystemInterfaces() ([]systemInterface, error) {
	return nil, errors.New("ioctl interface enumeration is only supported on linux and android")
}
