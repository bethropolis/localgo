package cli

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
)

// AnonymizedAlias returns a stable "Device #XXXXXXXX" identifier from a device's fingerprint.
func AnonymizedAlias(device *model.Device) string {
	if device == nil || device.Fingerprint == "" {
		return "Device #00000000"
	}
	h := sha256.Sum256([]byte(device.Fingerprint))
	return fmt.Sprintf("Device #%08x", h[:4])
}

// AnonymizeString returns a stable "Device #XXXXXXXX" identifier from any string.
func AnonymizeString(s string) string {
	if s == "" {
		return "Device #00000000"
	}
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("Device #%08x", h[:4])
}

// TruncateString truncates a string to maxLen characters
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// FormatBytes formats bytes in human readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	suffixes := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit && exp < len(suffixes)-1; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), suffixes[exp])
}

// FormatDuration formats duration in human readable format
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}
