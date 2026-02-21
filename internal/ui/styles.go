// styles.go defines shared Catppuccin Mocha color palette styles using
// charmbracelet/lipgloss. It provides Title (purple), Success (green), Fail (red),
// Warn (orange), Dim (gray), and Accent (pink) styles for consistent terminal
// output across the application.
package ui

import "github.com/charmbracelet/lipgloss"

// Shared Catppuccin Mocha color palette styles used across the application.
var (
	Title   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	Success = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	Fail    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
	Warn    = lipgloss.NewStyle().Foreground(lipgloss.Color("#fab387"))
	Dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	Accent  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f5c2e7"))
)
