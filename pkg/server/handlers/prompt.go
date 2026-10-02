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
	"golang.org/x/term"
)

// promptTimeout bounds how long an accept prompt waits for input.
// Expiry (or any prompt failure) fails closed: the transfer is rejected.
const promptTimeout = 30 * time.Second

// requireInteractiveTerminal reports whether an accept prompt can actually be
// answered. Without a terminal on stdin (piped/backgrounded server, service
// unit, non-TTY Termux session) huh would wait out its timeout invisible
// while the sender hangs — fail closed immediately instead.
func (h *ReceiveHandler) requireInteractiveTerminal() bool {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return true
	}
	h.logger.Warn("Rejecting transfer: no interactive terminal to prompt for acceptance (use --auto-accept or trusted fingerprints for headless receivers)")
	fmt.Fprintf(os.Stderr, "\n%s Transfer automatically rejected (no interactive terminal).\n", cli.WarningStyle.Render(cli.IconWarning))
	return false
}

// announcePrompt prints a plain-text banner on stderr before the interactive
// prompt runs, so the pending question stays visible even if the TUI render
// is lost among streaming server logs.
func announcePrompt(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n━━ "+format+" (default: reject after %ds) ━━\n", append(args, int(promptTimeout.Seconds()))...)
	os.Stderr.Sync()
}

func (h *ReceiveHandler) promptUserForAcceptance(sender model.DeviceInfo, files map[string]model.FileDto) bool {
	if cli.IsContainer() {
		return false
	}
	if !h.requireInteractiveTerminal() {
		return false
	}

	fileCount := len(files)
	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}

	cli.Notify("LocalGo: Incoming Transfer",
		fmt.Sprintf("%s wants to send you %d file(s) (%s)", cli.Sanitize(sender.Alias), fileCount, cli.FormatBytes(totalSize)))

	// Build a structured summary of the incoming files
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s (IP: %s)\n\nFiles:\n", cli.Sanitize(sender.Alias), sender.IP))

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
				sb.WriteString(fmt.Sprintf("  %s [Text] %s (%s)\n", cli.IconFile, cli.Sanitize(file.FileName), cli.FormatBytes(file.Size)))
			}
		} else {
			sb.WriteString(fmt.Sprintf("  %s %s (%s)\n", cli.IconFile, cli.Sanitize(file.FileName), cli.FormatBytes(file.Size)))
		}
		count++
	}

	if totalSize > 0 {
		sb.WriteString(fmt.Sprintf("\nTotal Size: %s", cli.FormatBytes(totalSize)))
	}

	var accept bool = true

	announcePrompt("Incoming file transfer from %s — answer the prompt below", cli.Sanitize(sender.Alias))
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

	ctx, cancel := context.WithTimeout(context.Background(), promptTimeout)
	defer cancel()

	err := form.RunWithContext(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s Transfer automatically rejected.\n", cli.WarningStyle.Render(cli.IconWarning))
		return false
	}

	return accept
}

func (h *ReceiveHandler) promptForClipboard(alias, remoteAddr, message string) bool {
	if cli.IsContainer() {
		return false
	}
	if !h.requireInteractiveTerminal() {
		return false
	}
	cli.Notify("LocalGo: Clipboard Message",
		fmt.Sprintf("%s sent clipboard text (%d chars)", cli.Sanitize(alias), len(message)))

	truncated := message
	if len(truncated) > 500 {
		truncated = truncated[:500] + "\n… (truncated)"
	}

	desc := fmt.Sprintf("From: %s (IP: %s)\n\nClipboard:\n%s", cli.Sanitize(alias), remoteAddr, cli.Sanitize(truncated))

	var accept bool = true
	announcePrompt("Incoming clipboard message from %s — answer the prompt below", cli.Sanitize(alias))
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Accept Clipboard?").
				Description(desc).
				Value(&accept).
				Affirmative("Accept & Copy").
				Negative("Reject"),
		),
	).WithTheme(huh.ThemeCharm())

	ctx, cancel := context.WithTimeout(context.Background(), promptTimeout)
	defer cancel()

	err := form.RunWithContext(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s Clipboard automatically rejected.\n", cli.WarningStyle.Render(cli.IconWarning))
		return false
	}
	return accept
}
