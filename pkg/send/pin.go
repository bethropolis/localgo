package send

import (
	"fmt"
	"os"
	"strings"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/charmbracelet/huh"
	"golang.org/x/term"
)

// promptForPIN asks the receiver's PIN on the terminal. It reports false
// when no prompt is possible (non-TTY, or --json where the caller must stay
// machine-readable), in which case a pin_required event is emitted for
// wrappers to act on.
func promptForPIN(reason string) (string, bool) {
	if cli.JSONMode() {
		cli.EmitEvent(cli.IPCEvent{Type: cli.EventPINRequired, Data: reason})
		return "", false
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", false
	}

	var pin string
	err := huh.NewInput().
		Title("PIN required").
		Description(fmt.Sprintf("%s — enter the PIN to continue", reason)).
		EchoMode(huh.EchoModePassword).
		CharLimit(64).
		Value(&pin).
		Run()
	if err != nil {
		return "", false
	}
	pin = strings.TrimSpace(pin)
	if pin == "" {
		return "", false
	}
	return pin, true
}
