package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/frontendtony/shepherd/internal/process"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.logViewport = viewport.New(m.logPanelInnerWidth(), m.panelContentHeight())
			m.ready = true
		} else {
			m.logViewport.Width = m.logPanelInnerWidth()
			m.logViewport.Height = m.panelContentHeight()
		}
		m.updateLogContent()

	case stateEventMsg:
		m.refreshStates()
		m.err = nil
		cmds = append(cmds, listenForEvents(m.manager))

	case tickMsg:
		m.refreshStates()
		m.updateLogContent()
		cmds = append(cmds, tickEvery())

	case errMsg:
		m.err = msg.error

	case tea.KeyMsg:
		cmd := m.handleKey(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	// Confirmation modes take priority.
	if m.confirmQuit {
		if msg.String() == "y" {
			m.manager.Shutdown()
			return tea.Quit
		}
		m.confirmQuit = false
		return nil
	}
	if m.confirmStopAll {
		if msg.String() == "y" {
			m.confirmStopAll = false
			return stopAllCmd(m.manager)
		}
		m.confirmStopAll = false
		return nil
	}

	// Help overlay.
	if m.showHelp {
		if key.Matches(msg, keys.Help) || msg.String() == "esc" {
			m.showHelp = false
		}
		return nil
	}

	// Log panel focused.
	if m.focusedPanel == PanelLogs {
		return m.handleLogPanelKey(msg)
	}

	// Process list focused.
	return m.handleProcessListKey(msg)
}

func (m *Model) handleLogPanelKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Tab):
		m.focusedPanel = PanelProcessList
	case key.Matches(msg, keys.Quit):
		return m.handleQuit()
	case key.Matches(msg, keys.Help):
		m.showHelp = true
	default:
		var cmd tea.Cmd
		m.logViewport, cmd = m.logViewport.Update(msg)
		m.autoScroll = m.logViewport.AtBottom()
		return cmd
	}
	return nil
}

func (m *Model) handleProcessListKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Quit):
		return m.handleQuit()
	case key.Matches(msg, keys.Help):
		m.showHelp = true
	case key.Matches(msg, keys.Up):
		if m.selectedIdx > 0 {
			m.selectedIdx--
			m.updateSelectedProc()
		}
	case key.Matches(msg, keys.Down):
		if m.selectedIdx < len(m.items)-1 {
			m.selectedIdx++
			m.updateSelectedProc()
		}
	case key.Matches(msg, keys.Enter):
		if m.selectedIdx < len(m.items) {
			item := m.items[m.selectedIdx]
			if item.isGroup {
				m.groups[item.groupIdx].expanded = !m.groups[item.groupIdx].expanded
				m.rebuildItems()
				if m.selectedIdx >= len(m.items) {
					m.selectedIdx = len(m.items) - 1
				}
			}
		}
	case key.Matches(msg, keys.Start):
		if m.selectedIdx < len(m.items) && !m.items[m.selectedIdx].isGroup {
			return startProcessCmd(m.manager, m.items[m.selectedIdx].name)
		}
	case key.Matches(msg, keys.Stop):
		if m.selectedIdx < len(m.items) && !m.items[m.selectedIdx].isGroup {
			return stopProcessCmd(m.manager, m.items[m.selectedIdx].name)
		}
	case key.Matches(msg, keys.Restart):
		if m.selectedIdx < len(m.items) && !m.items[m.selectedIdx].isGroup {
			return restartProcessCmd(m.manager, m.items[m.selectedIdx].name)
		}
	case key.Matches(msg, keys.StartGrp):
		if g := m.selectedGroup(); g != nil {
			return startGroupCmd(m.manager, g.processes)
		}
	case key.Matches(msg, keys.StopGrp):
		if g := m.selectedGroup(); g != nil {
			return stopGroupCmd(m.manager, g.processes)
		}
	case key.Matches(msg, keys.StartAll):
		return startAllCmd(m.manager, m.config)
	case key.Matches(msg, keys.StopAll):
		if m.countByStatus(process.StatusRunning) > 0 {
			m.confirmStopAll = true
		}
	case key.Matches(msg, keys.Tab), key.Matches(msg, keys.Logs):
		m.focusedPanel = PanelLogs
	}
	return nil
}

func (m *Model) handleQuit() tea.Cmd {
	running := 0
	for _, s := range m.states {
		if s.Status == process.StatusRunning || s.Status == process.StatusStarting ||
			s.Status == process.StatusRetrying {
			running++
		}
	}
	if running > 0 {
		m.confirmQuit = true
		return nil
	}
	return tea.Quit
}

func (m *Model) updateSelectedProc() {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.items) {
		item := m.items[m.selectedIdx]
		if !item.isGroup {
			m.selectedProc = item.name
			m.autoScroll = true
			m.updateLogContent()
		}
	}
}

func (m Model) selectedGroup() *groupView {
	if m.selectedIdx >= len(m.items) {
		return nil
	}
	item := m.items[m.selectedIdx]
	idx := item.groupIdx
	if idx >= 0 && idx < len(m.groups) {
		return &m.groups[idx]
	}
	return nil
}

func (m Model) listPanelWidth() int {
	w := m.width * 2 / 5
	if w < 25 {
		w = 25
	}
	return w
}

func (m Model) logPanelWidth() int {
	return m.width - m.listPanelWidth()
}

func (m Model) logPanelInnerWidth() int {
	return m.logPanelWidth() - 4
}

func (m Model) panelContentHeight() int {
	return m.height - 3
}
