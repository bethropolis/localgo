package cli

import "fmt"

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
