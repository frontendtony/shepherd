package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/frontendtony/shepherd/internal/process"
)

type groupView struct {
	name      string
	expanded  bool
	processes []string
}

type listItem struct {
	isGroup   bool
	name      string
	groupName string
	groupIdx  int
}

func (m Model) renderProcessList(width, height int) string {
	focused := m.focusedPanel == PanelProcessList
	innerWidth := width - 2 // border

	var lines []string

	for i, item := range m.items {
		var line string

		if item.isGroup {
			line = m.renderGroupRow(item, innerWidth)
		} else {
			line = m.renderProcessRow(item, innerWidth)
		}

		if i == m.selectedIdx && focused {
			line = lipgloss.NewStyle().
				Bold(true).
				Background(colorAccent).
				Foreground(lipgloss.Color("#FFFFFF")).
				Width(innerWidth).
				Render(line)
		}

		lines = append(lines, line)
	}

	contentHeight := height - 2
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}

	content := strings.Join(lines, "\n")

	borderColor := colorSubtle
	if focused {
		borderColor = colorAccent
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(innerWidth).
		Height(contentHeight).
		Render(content)
}

func (m Model) renderGroupRow(item listItem, width int) string {
	g := m.groups[item.groupIdx]
	arrow := "▼"
	if !g.expanded {
		arrow = "▶"
	}

	running := 0
	for _, p := range g.processes {
		if s, ok := m.states[p]; ok && s.Status == process.StatusRunning {
			running++
		}
	}
	total := len(g.processes)

	return fmt.Sprintf(" %s %s (%d/%d)", arrow, g.name, running, total)
}

func (m Model) renderProcessRow(item listItem, width int) string {
	state := m.states[item.name]
	icon := statusIcon(state.Status)
	styledIcon := statusStyle(state.Status).Render(icon)

	info := string(state.Status)
	if state.Status == process.StatusRunning {
		info = formatUptime(state.Uptime())
	} else if state.Status == process.StatusRetrying {
		info = fmt.Sprintf("retry #%d", state.RetryCount)
	}

	styledInfo := statusStyle(state.Status).Render(info)
	infoWidth := lipgloss.Width(styledInfo)

	name := item.name
	maxName := width - 8 - infoWidth
	if maxName < 5 {
		maxName = 5
	}
	if len(name) > maxName {
		name = name[:maxName-1] + "…"
	}

	padding := width - 6 - len(name) - infoWidth
	if padding < 1 {
		padding = 1
	}

	return fmt.Sprintf("   %s %s%s%s", styledIcon, name, strings.Repeat(" ", padding), styledInfo)
}
