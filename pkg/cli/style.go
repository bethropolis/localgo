package cli

import (
	"github.com/charmbracelet/lipgloss"
)

// UI Styles
var (
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Bold(true) // Green
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // Red
	WarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true) // Orange/Yellow
	InfoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)  // Blue
	HeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Underline(true) // Pinkish
	MutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))            // Grey
	HighlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true) // Yellow
)
