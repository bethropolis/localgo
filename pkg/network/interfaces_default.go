//go:build !android

package network

import "fmt"

// platformInterfaceFallback reports the enumeration error unchanged.
// Only android carries an ioctl-based fallback (interfaces_android.go);
// everywhere else a net.Interfaces() failure is a genuine error.
func platformInterfaceFallback(origErr error) ([]systemInterface, error) {
	return nil, fmt.Errorf("failed to get network interfaces: %w", origErr)
}
