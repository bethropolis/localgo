package handlers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/charmbracelet/huh"
)

func (h *ReceiveHandler) promptUserForAcceptance(sender model.DeviceInfo, files map[string]model.FileDto) bool {
	if cli.IsContainer() {
		return false
	}

	fileCount := len(files)
	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}

	cli.Notify("LocalGo: Incoming Transfer",
		fmt.Sprintf("%s wants to send you %d file(s) (%s)", sender.Alias, fileCount, cli.FormatBytes(totalSize)))

	// Build a structured summary of the incoming files
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s (IP: %s)\n\nFiles:\n", sender.Alias, sender.IP))

	count := 0
	for _, file := range files {
		if count >= 5 {
			sb.WriteString(fmt.Sprintf("  ... and %d more files\n", fileCount-5))
			break
		}
		isText := strings.HasPrefix(file.FileType, "text/plain")
		if isText {
			preview := ""
			if file.Preview != nil && *file.Preview != "" {
				preview = *file.Preview
				if len(preview) > 50 {
					preview = preview[:50] + "…"
				}
				sb.WriteString(fmt.Sprintf("  %s [Text] %q\n", cli.IconFile, preview))
			} else {
				sb.WriteString(fmt.Sprintf("  %s [Text] %s (%s)\n", cli.IconFile, file.FileName, cli.FormatBytes(file.Size)))
			}
		} else {
			sb.WriteString(fmt.Sprintf("  %s %s (%s)\n", cli.IconFile, file.FileName, cli.FormatBytes(file.Size)))
		}
		count++
	}

	if totalSize > 0 {
		sb.WriteString(fmt.Sprintf("\nTotal Size: %s", cli.FormatBytes(totalSize)))
	}

	var accept bool = true

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Accept Incoming File Transfer?").
				Description(sb.String()).
				Value(&accept).
				Affirmative("Accept").
				Negative("Reject"),
		),
	).WithTheme(huh.ThemeCharm())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := form.RunWithContext(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s Transfer automatically rejected.\n", cli.WarningStyle.Render(cli.IconWarning))
		return false
	}

	return accept
}
