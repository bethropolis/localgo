package cmd

import (
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/model"
)

// padRight pads a string with spaces on the right up to the specified length,
// correctly ignoring ANSI color escape codes for width calculation.
func padRight(str string, length int) string {
	plain := stripansi.Strip(str)
	if len(plain) >= length {
		return str
	}
	return str + strings.Repeat(" ", length-len(plain))
}

// anonymizeDeviceSlice returns a copy of devices with anonymized aliases for private mode.
func anonymizeDeviceSlice(devices []*model.Device) []*model.Device {
	out := make([]*model.Device, len(devices))
	for i, d := range devices {
		copy := *d
		copy.Alias = cli.AnonymizedAlias(d)
		out[i] = &copy
	}
	return out
}

func displayDevices(devices []*model.Device, jsonOutput bool, quiet bool, method string) error {
	if Cfg != nil && Cfg.Private {
		devices = anonymizeDeviceSlice(devices)
	}
	format := cli.FormatTable
	if jsonOutput {
		format = cli.FormatJSON
	} else if quiet {
		format = cli.FormatQuiet
	}

	writer := cli.NewOutputWriter(format)
	defer writer.Flush()

	return writer.WriteDevices(devices, method)
}
