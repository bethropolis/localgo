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

func displayDevices(devices []*model.Device, jsonOutput bool, quiet bool, method string) error {
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
