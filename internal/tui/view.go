package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.showHelp {
		return m.renderHelp()
	}

	leftWidth := m.listPanelWidth()
	rightWidth := m.logPanelWidth()
	panelHeight := m.height - 1

	left := m.renderProcessList(leftWidth, panelHeight)
	right := m.renderLogPanel(rightWidth, panelHeight)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	status := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, panels, status)
}
