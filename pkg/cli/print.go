package cli

import "fmt"

// All human-readable stdout printers no-op in JSON mode so stdout carries
// pure NDJSON events (see events.go).

func PrintSuccess(format string, a ...any) {
	if JSONMode() {
		return
	}
	fmt.Println(SuccessStyle.Render(IconCheck + " " + fmt.Sprintf(format, a...)))
}

func PrintError(format string, a ...any) {
	if JSONMode() {
		return
	}
	fmt.Println(ErrorStyle.Render(IconCross + " " + fmt.Sprintf(format, a...)))
}

func PrintWarning(format string, a ...any) {
	if JSONMode() {
		return
	}
	fmt.Println(WarningStyle.Render(IconWarning + " " + fmt.Sprintf(format, a...)))
}

func PrintInfo(format string, a ...any) {
	if JSONMode() {
		return
	}
	fmt.Println(InfoStyle.Render(IconInfo + " " + fmt.Sprintf(format, a...)))
}

func PrintHeader(text string) {
	if JSONMode() {
		return
	}
	fmt.Println(HeaderStyle.Render(text))
}
