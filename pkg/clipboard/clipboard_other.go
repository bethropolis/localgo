//go:build !linux && !darwin && !windows

package clipboard

// detect returns nil on unsupported platforms (Android, plan9, etc.).
func detect() *clipProvider {
	return nil
}
