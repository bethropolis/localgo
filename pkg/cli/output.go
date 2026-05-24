package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bethropolis/localgo/pkg/model"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/gen2brain/beeep"
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
	fmt.Fprintf(os.Stderr, "%s\n", ErrorStyle.Render(fmt.Sprintf("%s Error: %v", IconCross, err)))
}

// WriteProgress outputs progress information
func (ow *OutputWriter) WriteProgress(message string) {
	if ow.format != FormatQuiet {
		fmt.Printf("%s\n", InfoStyle.Render(fmt.Sprintf("%s %s", IconSpinner, message)))
	}
}

// WriteSuccess outputs a success message
func (ow *OutputWriter) WriteSuccess(message string) {
	if ow.format != FormatQuiet {
		fmt.Printf("%s\n", SuccessStyle.Render(fmt.Sprintf("%s %s", IconCheck, message)))
	}
}

// WriteWarning outputs a warning message
func (ow *OutputWriter) WriteWarning(message string) {
	if ow.format != FormatQuiet {
		fmt.Printf("%s\n", WarningStyle.Render(fmt.Sprintf("%s %s", IconWarning, message)))
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

// writeDeviceInfoTable outputs device info in a stylized card layout
func (ow *OutputWriter) writeDeviceInfoTable(info DeviceInfo) error {
	titleStyle := HeaderStyle.Padding(0, 1).MarginBottom(1)
	borderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(1, 2)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Width(18).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))

	fields := []struct {
		label string
		value string
	}{
		{"Alias", info.Alias},
		{"Protocol", "LocalSend v" + info.Version},
		{"Device Model", info.DeviceModel},
		{"Device Type", info.DeviceType},
		{"Port", fmt.Sprintf("%d", info.Port)},
		{"Transport", strings.ToUpper(info.Protocol)},
		{"Download Dir", info.DownloadDir},
		{"PIN Protection", func() string {
			if info.HasPin {
				return "Enabled"
			}
			return "Disabled"
		}()},
		{"Multicast", info.MulticastAddr},
		{"Fingerprint", info.Fingerprint},
	}

	var content strings.Builder
	for _, f := range fields {
		content.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render(f.label+":"), valueStyle.Render(f.value)))
	}

	fmt.Println(titleStyle.Render(IconDevice + "  LocalGo Device Information"))
	fmt.Println(borderStyle.Render(content.String()))
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

// Notify sends a native desktop notification. Icon is empty (system default).
// No-op in container environments.
func Notify(title, body string) {
	if IsContainer() {
		return
	}
	beeep.Notify(title, body, "")
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

// Standalone Print helpers

func PrintSuccess(format string, a ...any) {
	fmt.Println(SuccessStyle.Render(IconCheck + " " + fmt.Sprintf(format, a...)))
}

func PrintError(format string, a ...any) {
	fmt.Println(ErrorStyle.Render(IconCross + " " + fmt.Sprintf(format, a...)))
}

func PrintWarning(format string, a ...any) {
	fmt.Println(WarningStyle.Render(IconWarning + " " + fmt.Sprintf(format, a...)))
}

func PrintInfo(format string, a ...any) {
	fmt.Println(InfoStyle.Render(IconInfo + " " + fmt.Sprintf(format, a...)))
}

func PrintHeader(text string) {
	fmt.Println(HeaderStyle.Render(text))
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

// PickDevice presents an interactive TUI to select a device. Returns the selected device or nil if canceled.
func PickDevice(devices []*model.Device) *model.Device {
	if IsContainer() {
		return nil
	}
	if len(devices) == 0 {
		return nil
	}
	if len(devices) == 1 {
		return devices[0]
	}

	var selected *model.Device
	options := make([]huh.Option[*model.Device], len(devices))
	for i, d := range devices {
		protocol := strings.ToUpper(string(d.Protocol))
		options[i] = huh.NewOption(
			fmt.Sprintf("%s  %s:%d  [%s]", d.Alias, d.IP, d.Port, protocol),
			d,
		)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[*model.Device]().
				Title("Select recipient:").
				Options(options...).
				Value(&selected).
				WithHeight(10),
		),
	)
	form.Run()
	return selected
}


