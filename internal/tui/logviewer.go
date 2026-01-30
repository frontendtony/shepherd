package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderLogPanel(width, height int) string {
	focused := m.focusedPanel == PanelLogs
	innerWidth := width - 2

	borderColor := colorSubtle
	if focused {
		borderColor = colorAccent
	}

	contentHeight := height - 2

	var content string
	if m.selectedProc == "" || !m.ready {
		content = lipgloss.NewStyle().
			Foreground(colorDim).
			Render("Select a process to view logs")
	} else {
		content = m.logViewport.View()
	}

	// Show scroll indicator when not at bottom
	if m.ready && focused && m.selectedProc != "" && !m.logViewport.AtBottom() {
		indicator := lipgloss.NewStyle().
			Foreground(colorAccent).
			Render("  ↓ new output below")
		lines := strings.Split(content, "\n")
		if len(lines) > 0 {
			lines[len(lines)-1] = indicator
			content = strings.Join(lines, "\n")
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(innerWidth).
		Height(contentHeight).
		Render(content)
}

func (m *Model) updateLogContent() {
	if m.selectedProc == "" || !m.ready {
		return
	}
	buf := m.manager.GetLogBuffer(m.selectedProc)
	if buf == nil {
		m.logViewport.SetContent("No logs available")
		return
	}
	lines := buf.All()
	if len(lines) == 0 {
		m.logViewport.SetContent(
			lipgloss.NewStyle().Foreground(colorDim).Render("No output yet"),
		)
		return
	}
	m.logViewport.SetContent(strings.Join(lines, "\n"))
	if m.autoScroll {
		m.logViewport.GotoBottom()
	}
}
