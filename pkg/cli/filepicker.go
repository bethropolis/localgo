package cli

import (
	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
)

// FilePickerModel wraps bubbles/filepicker.Model as a tea.Model for TUI file selection.
type FilePickerModel struct {
	fp   filepicker.Model
	File string
	quit bool
}

func (m FilePickerModel) Init() tea.Cmd {
	return m.fp.Init()
}

func (m FilePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quit = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.fp, cmd = m.fp.Update(msg)
	if m.fp.Path != "" {
		m.File = m.fp.Path
		return m, tea.Quit
	}
	return m, cmd
}

func (m FilePickerModel) View() string {
	if m.quit {
		return ""
	}
	return m.fp.View()
}

// LaunchFilePicker opens an interactive TUI file browser and returns the selected file path.
// Returns empty string if the user cancelled.
func LaunchFilePicker() (string, error) {
	fp := filepicker.New()
	fp.DirAllowed = true
	fp.FileAllowed = true
	fp.ShowHidden = false

	m := FilePickerModel{fp: fp}
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}
	if picked, ok := result.(FilePickerModel); ok {
		return picked.File, nil
	}
	return "", nil
}
