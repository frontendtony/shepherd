package tui

import (
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/frontendtony/shepherd/internal/config"
	"github.com/frontendtony/shepherd/internal/process"
)

// Panel represents which UI panel has focus.
type Panel int

const (
	PanelProcessList Panel = iota
	PanelLogs
)

// Custom Bubble Tea messages.
type stateEventMsg process.StateEvent
type tickMsg time.Time
type errMsg struct{ error }

// ConfigReloadMsg is sent when config is reloaded via SIGHUP.
type ConfigReloadMsg struct {
	Config *config.Config
}

// NotifyMsg is sent to display a temporary notification in the status bar.
type NotifyMsg struct {
	Text string
}

// Model is the main Bubble Tea model for the shepherd TUI.
type Model struct {
	manager *process.ProcessManager
	config  *config.Config

	groups      []groupView
	items       []listItem
	states      map[string]process.ProcessState
	selectedIdx int

	focusedPanel   Panel
	selectedProc   string
	logViewport    viewport.Model
	autoScroll     bool
	showHelp       bool
	fullScreenLogs bool
	confirmQuit    bool
	confirmStopAll bool
	width, height  int

	autoStart    string
	err          error
	errSetAt     time.Time
	notification string
	notifyUntil  time.Time
	ready        bool
}

// NewModel creates the TUI model wired to the given process manager.
func NewModel(mgr *process.ProcessManager, cfg *config.Config, autoStart string) Model {
	m := Model{
		manager:      mgr,
		config:       cfg,
		autoStart:    autoStart,
		autoScroll:   true,
		states:       make(map[string]process.ProcessState),
		focusedPanel: PanelProcessList,
	}

	m.buildGroups()
	m.rebuildItems()
	m.refreshStates()

	// Select first process (skip group headers).
	for i, item := range m.items {
		if !item.isGroup {
			m.selectedIdx = i
			m.selectedProc = item.name
			break
		}
	}

	return m
}

func (m *Model) buildGroups() {
	grouped := make(map[string]bool)

	var groupNames []string
	for name := range m.config.Groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	for _, name := range groupNames {
		g := m.config.Groups[name]
		m.groups = append(m.groups, groupView{
			name:      name,
			expanded:  true,
			processes: g.Processes,
		})
		for _, p := range g.Processes {
			grouped[p] = true
		}
	}

	// Ungrouped processes go into "other".
	var ungrouped []string
	for name := range m.config.Processes {
		if !grouped[name] {
			ungrouped = append(ungrouped, name)
		}
	}
	if len(ungrouped) > 0 {
		sort.Strings(ungrouped)
		m.groups = append(m.groups, groupView{
			name:      "other",
			expanded:  true,
			processes: ungrouped,
		})
	}
}

func (m *Model) rebuildItems() {
	m.items = nil
	for i, g := range m.groups {
		m.items = append(m.items, listItem{
			isGroup:  true,
			name:     g.name,
			groupIdx: i,
		})
		if g.expanded {
			for _, p := range g.processes {
				m.items = append(m.items, listItem{
					name:      p,
					groupName: g.name,
					groupIdx:  i,
				})
			}
		}
	}
}

func (m *Model) refreshStates() {
	for _, s := range m.manager.GetAllStates() {
		m.states[s.Name] = s
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		listenForEvents(m.manager),
		tickEvery(),
	}
	if m.autoStart != "" {
		cmds = append(cmds, startByNameCmd(m.manager, m.autoStart))
	}
	return tea.Batch(cmds...)
}

// Tea commands

func listenForEvents(mgr *process.ProcessManager) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-mgr.Events()
		if !ok {
			return nil
		}
		return stateEventMsg(event)
	}
}

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func startByNameCmd(mgr *process.ProcessManager, name string) tea.Cmd {
	return func() tea.Msg {
		if err := mgr.StartByName(name); err != nil {
			return errMsg{err}
		}
		return nil
	}
}

func startProcessCmd(mgr *process.ProcessManager, name string) tea.Cmd {
	return func() tea.Msg {
		if err := mgr.StartProcess(name); err != nil {
			return errMsg{err}
		}
		return nil
	}
}

func stopProcessCmd(mgr *process.ProcessManager, name string) tea.Cmd {
	return func() tea.Msg {
		if err := mgr.StopProcess(name); err != nil {
			return errMsg{err}
		}
		return nil
	}
}

func restartProcessCmd(mgr *process.ProcessManager, name string) tea.Cmd {
	return func() tea.Msg {
		if err := mgr.RestartProcess(name); err != nil {
			return errMsg{err}
		}
		return nil
	}
}

func startGroupCmd(mgr *process.ProcessManager, processes []string) tea.Cmd {
	return func() tea.Msg {
		for _, name := range processes {
			if err := mgr.StartProcess(name); err != nil {
				return errMsg{err}
			}
		}
		return nil
	}
}

func stopGroupCmd(mgr *process.ProcessManager, processes []string) tea.Cmd {
	return func() tea.Msg {
		for _, name := range processes {
			if err := mgr.StopProcess(name); err != nil {
				return errMsg{err}
			}
		}
		return nil
	}
}

func startAllCmd(mgr *process.ProcessManager, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		for name := range cfg.Processes {
			if err := mgr.StartProcess(name); err != nil {
				return errMsg{err}
			}
		}
		return nil
	}
}

func stopAllCmd(mgr *process.ProcessManager) tea.Cmd {
	return func() tea.Msg {
		if err := mgr.StopAll(); err != nil {
			return errMsg{err}
		}
		return nil
	}
}
