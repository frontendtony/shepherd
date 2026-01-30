package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/frontendtony/shepherd/internal/process"
)

func (m Model) renderStatusBar() string {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color("#333333")).
		Foreground(lipgloss.Color("#FFFFFF"))

	if m.confirmQuit {
		running := m.countByStatus(process.StatusRunning)
		return style.Width(m.width).Render(fmt.Sprintf(" %d process(es) running. Quit? (y/n)", running))
	}
	if m.confirmStopAll {
		running := m.countByStatus(process.StatusRunning)
		return style.Width(m.width).Render(fmt.Sprintf(" Stop all %d process(es)? (y/n)", running))
	}

	if m.err != nil {
		return style.Copy().
			Background(lipgloss.Color("#E74C3C")).
			Width(m.width).
			Render(fmt.Sprintf(" Error: %s", m.err.Error()))
	}

	if m.notification != "" {
		return style.Copy().
			Background(lipgloss.Color("#2ECC71")).
			Foreground(lipgloss.Color("#000000")).
			Width(m.width).
			Render(fmt.Sprintf(" %s", m.notification))
	}

	running := m.countByStatus(process.StatusRunning)
	total := len(m.states)
	left := fmt.Sprintf(" %d/%d running", running, total)

	var hints []string
	if m.focusedPanel == PanelProcessList {
		hints = append(hints, "↑/↓ navigate", "s start", "x stop", "r restart", "? help")
	} else {
		hints = append(hints, "↑/↓ scroll", "tab back", "? help")
	}
	right := strings.Join(hints, "  ") + " "

	padding := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 1 {
		padding = 1
	}

	return style.Width(m.width).Render(left + strings.Repeat(" ", padding) + right)
}

func (m Model) countByStatus(status process.Status) int {
	count := 0
	for _, s := range m.states {
		if s.Status == status {
			count++
		}
	}
	return count
}
