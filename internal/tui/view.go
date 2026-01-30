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

	if m.fullScreenLogs {
		return m.renderFullScreenLogs()
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

func (m Model) renderFullScreenLogs() string {
	header := "Logs"
	if m.selectedProc != "" {
		state := m.states[m.selectedProc]
		header = "Logs: " + m.selectedProc + " [" + string(state.Status) + "]"
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorAccent)

	footer := lipgloss.NewStyle().
		Foreground(colorDim).
		Render("f close  ↑/↓ scroll  q quit")

	contentHeight := m.height - 3 // header + footer + border spacing
	content := m.logViewport.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		headerStyle.Render(header),
		lipgloss.NewStyle().Height(contentHeight).Render(content),
		footer,
	)
}
