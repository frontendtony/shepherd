package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	Start      key.Binding
	Stop       key.Binding
	Restart    key.Binding
	StartGrp   key.Binding
	StopGrp    key.Binding
	StartAll   key.Binding
	StopAll    key.Binding
	Tab        key.Binding
	Logs       key.Binding
	FullScreen key.Binding
	Help       key.Binding
	Quit       key.Binding
}

var keys = keyMap{
	Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Enter:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "expand/collapse")),
	Start:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
	Stop:       key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop")),
	Restart:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")),
	StartGrp:   key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "start group")),
	StopGrp:    key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "stop group")),
	StartAll:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "start all")),
	StopAll:    key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "stop all")),
	Tab:        key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch panel")),
	Logs:       key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "view logs")),
	FullScreen: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "fullscreen logs")),
	Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
