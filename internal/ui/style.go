package ui

import "github.com/charmbracelet/lipgloss"

var (
	Title  = lipgloss.NewStyle().Bold(true)
	OK     = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).SetString("✓")
	Fail   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).SetString("✗")
	Dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	Warn   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	KeyCol = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)

// Checkline renders one status line, e.g. "✓  git  2.43.0".
func Checkline(label string, ok bool, detail string) string {
	mark := OK.String()
	if !ok {
		mark = Fail.String()
	}
	if detail == "" {
		return mark + "  " + label
	}
	return mark + "  " + label + "  " + Dim.Render(detail)
}
