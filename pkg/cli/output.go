package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bet/localgo/pkg/model"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	FormatJSON  OutputFormat = "json"
	FormatTable OutputFormat = "table"
	FormatQuiet OutputFormat = "quiet"
)

// OutputWriter handles different output formats
type OutputWriter struct {
	format OutputFormat
	writer *tabwriter.Writer
}

// NewOutputWriter creates a new output writer
func NewOutputWriter(format OutputFormat) *OutputWriter {
	return &OutputWriter{
		format: format,
		writer: tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0),
	}
}

// WriteDevices outputs a list of devices in the specified format
func (ow *OutputWriter) WriteDevices(devices []*model.Device, method string) error {
	switch ow.format {
	case FormatJSON:
		return ow.writeDevicesJSON(devices)
	case FormatQuiet:
		return ow.writeDevicesQuiet(devices)
	default:
		return ow.writeDevicesTable(devices, method)
	}
}

// WriteDeviceInfo outputs device information
func (ow *OutputWriter) WriteDeviceInfo(info DeviceInfo) error {
	switch ow.format {
	case FormatJSON:
		return ow.writeJSON(info)
	default:
		return ow.writeDeviceInfoTable(info)
	}
}

// WriteMessage outputs a simple message
func (ow *OutputWriter) WriteMessage(message string) {
	if ow.format != FormatQuiet {
		fmt.Println(message)
	}
}

// WriteError outputs an error message
func (ow *OutputWriter) WriteError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}

// WriteProgress outputs progress information
func (ow *OutputWriter) WriteProgress(message string) {
	if ow.format != FormatQuiet {
		fmt.Printf("⏳ %s\n", message)
	}
}

// WriteSuccess outputs a success message
func (ow *OutputWriter) WriteSuccess(message string) {
	if ow.format != FormatQuiet {
		fmt.Printf("✅ %s\n", message)
	}
}

// WriteWarning outputs a warning message
func (ow *OutputWriter) WriteWarning(message string) {
	if ow.format != FormatQuiet {
		fmt.Printf("⚠️  %s\n", message)
	}
}

// Flush flushes the output writer
func (ow *OutputWriter) Flush() error {
	if ow.writer != nil {
		return ow.writer.Flush()
	}
	return nil
}

// writeDevicesJSON outputs devices in JSON format
func (ow *OutputWriter) writeDevicesJSON(devices []*model.Device) error {
	return ow.writeJSON(map[string]interface{}{
		"devices":   devices,
		"count":     len(devices),
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// writeDevicesTable outputs devices in table format
func (ow *OutputWriter) writeDevicesTable(devices []*model.Device, method string) error {
	if len(devices) == 0 {
		fmt.Printf("No devices found via %s\n", method)
		return nil
	}

	fmt.Printf("Found %d device(s) via %s:\n\n", len(devices), method)

	// Write header
	fmt.Fprintf(ow.writer, "ALIAS\tIP ADDRESS\tPROTOCOL\tPORT\tDEVICE TYPE\tFINGERPRINT\n")
	fmt.Fprintf(ow.writer, "-----\t----------\t--------\t----\t-----------\t-----------\n")

	// Write devices
	for _, device := range devices {
		fmt.Fprintf(ow.writer, "%s\t%s\t%s\t%d\t%s\t%s...\n",
			truncateString(device.Alias, 20),
			device.IP,
			strings.ToUpper(string(device.Protocol)),
			device.Port,
			string(device.DeviceType),
			device.Fingerprint[:16],
		)
	}

	return ow.writer.Flush()
}

// writeDevicesQuiet outputs devices in quiet format (tab-separated)
func (ow *OutputWriter) writeDevicesQuiet(devices []*model.Device) error {
	for _, device := range devices {
		fmt.Printf("%s\t%s\t%s\t%d\t%s\n",
			device.Alias,
			device.IP,
			device.Protocol,
			device.Port,
			device.Fingerprint[:16]+"...")
	}
	return nil
}

// DeviceInfo represents device information for output
type DeviceInfo struct {
	Alias         string `json:"alias"`
	Version       string `json:"version"`
	DeviceModel   string `json:"deviceModel"`
	DeviceType    string `json:"deviceType"`
	Fingerprint   string `json:"fingerprint"`
	Port          int    `json:"port"`
	Protocol      string `json:"protocol"`
	DownloadDir   string `json:"downloadDir"`
	HasPin        bool   `json:"hasPin"`
	MulticastAddr string `json:"multicastAddr"`
}

// writeDeviceInfoTable outputs device info in table format
func (ow *OutputWriter) writeDeviceInfoTable(info DeviceInfo) error {
	fmt.Println("LocalGo Device Information")
	fmt.Println("==========================")
	fmt.Printf("Alias:           %s\n", info.Alias)
	fmt.Printf("Protocol:        LocalSend v%s\n", info.Version)
	fmt.Printf("Device Model:    %s\n", info.DeviceModel)
	fmt.Printf("Device Type:     %s\n", info.DeviceType)
	fmt.Printf("Port:            %d\n", info.Port)
	fmt.Printf("Transport:       %s\n", strings.ToUpper(info.Protocol))
	fmt.Printf("Download Dir:    %s\n", info.DownloadDir)
	fmt.Printf("PIN Protection:  %s\n", func() string {
		if info.HasPin {
			return "Enabled"
		}
		return "Disabled"
	}())
	fmt.Printf("Multicast:       %s\n", info.MulticastAddr)
	fmt.Printf("Fingerprint:     %s\n", info.Fingerprint)

	return nil
}

// writeJSON outputs data in JSON format
func (ow *OutputWriter) writeJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Helper functions

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
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
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
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

// ProgressBar represents a simple progress bar
type ProgressBar struct {
	total   int64
	current int64
	width   int
	prefix  string
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int64, prefix string) *ProgressBar {
	return &ProgressBar{
		total:  total,
		width:  50,
		prefix: prefix,
	}
}

// Update updates the progress bar
func (pb *ProgressBar) Update(current int64) {
	pb.current = current
	pb.render()
}

// Finish completes the progress bar
func (pb *ProgressBar) Finish() {
	pb.current = pb.total
	pb.render()
	fmt.Println()
}

// render renders the progress bar
func (pb *ProgressBar) render() {
	percent := float64(pb.current) / float64(pb.total)
	filled := int(percent * float64(pb.width))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", pb.width-filled)

	fmt.Printf("\r%s [%s] %.1f%% (%s/%s)",
		pb.prefix,
		bar,
		percent*100,
		FormatBytes(pb.current),
		FormatBytes(pb.total))
}

// ColorCode represents ANSI color codes
type ColorCode string

const (
	ColorReset  ColorCode = "\033[0m"
	ColorRed    ColorCode = "\033[31m"
	ColorGreen  ColorCode = "\033[32m"
	ColorYellow ColorCode = "\033[33m"
	ColorBlue   ColorCode = "\033[34m"
	ColorPurple ColorCode = "\033[35m"
	ColorCyan   ColorCode = "\033[36m"
	ColorWhite  ColorCode = "\033[37m"
	ColorBold   ColorCode = "\033[1m"
)

// Colorize applies color to text if stdout is a terminal
func Colorize(text string, color ColorCode) string {
	if isTerminal() {
		return string(color) + text + string(ColorReset)
	}
	return text
}

// isTerminal checks if stdout is a terminal
func isTerminal() bool {
	// Simple check - in a real implementation you might want to use
	// a library like github.com/mattn/go-isatty
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
