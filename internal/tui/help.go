package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderHelp() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorAccent).
		Render("Shepherd - Keybindings")

	sections := []struct {
		header   string
		bindings []string
	}{
		{
			header: "Navigation",
			bindings: []string{
				"↑/k     Move up",
				"↓/j     Move down",
				"Enter   Expand/collapse group",
				"Tab     Switch panel focus",
				"l       Focus log panel",
				"f       Fullscreen logs",
			},
		},
		{
			header: "Process Control",
			bindings: []string{
				"s       Start selected process",
				"x       Stop selected process",
				"r       Restart selected process",
			},
		},
		{
			header: "Group/All Control",
			bindings: []string{
				"g       Start all in group",
				"G       Stop all in group",
				"a       Start all processes",
				"X       Stop all processes",
			},
		},
		{
			header: "Other",
			bindings: []string{
				"?       Toggle this help",
				"q       Quit",
			},
		},
	}

	var parts []string
	parts = append(parts, title, "")

	for _, s := range sections {
		header := lipgloss.NewStyle().Bold(true).Render(s.header)
		parts = append(parts, header)
		for _, b := range s.bindings {
			parts = append(parts, "  "+b)
		}
		parts = append(parts, "")
	}

	parts = append(parts, lipgloss.NewStyle().Foreground(colorDim).Render("Press ? or Esc to close"))

	content := strings.Join(parts, "\n")

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 3).
			Render(content),
	)
}
