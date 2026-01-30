package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/frontendtony/shepherd/internal/process"
)

var (
	colorRunning  = lipgloss.AdaptiveColor{Light: "#2ECC71", Dark: "#2ECC71"}
	colorFailed   = lipgloss.AdaptiveColor{Light: "#E74C3C", Dark: "#E74C3C"}
	colorRetrying = lipgloss.AdaptiveColor{Light: "#F39C12", Dark: "#F39C12"}
	colorStopped  = lipgloss.AdaptiveColor{Light: "#7F8C8D", Dark: "#7F8C8D"}
	colorStarting = lipgloss.AdaptiveColor{Light: "#3498DB", Dark: "#3498DB"}

	colorAccent = lipgloss.AdaptiveColor{Light: "#10B981", Dark: "#10B981"}
	colorSubtle = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#666666"}
	colorDim    = lipgloss.AdaptiveColor{Light: "#999999", Dark: "#555555"}
)

func statusStyle(status process.Status) lipgloss.Style {
	switch status {
	case process.StatusRunning:
		return lipgloss.NewStyle().Foreground(colorRunning)
	case process.StatusFailed:
		return lipgloss.NewStyle().Foreground(colorFailed)
	case process.StatusRetrying:
		return lipgloss.NewStyle().Foreground(colorRetrying)
	case process.StatusStarting:
		return lipgloss.NewStyle().Foreground(colorStarting)
	default:
		return lipgloss.NewStyle().Foreground(colorStopped)
	}
}

func statusIcon(status process.Status) string {
	switch status {
	case process.StatusRunning:
		return "●"
	case process.StatusStopped:
		return "○"
	case process.StatusFailed:
		return "✗"
	case process.StatusRetrying:
		return "↻"
	case process.StatusStarting:
		return "◐"
	case process.StatusStopping:
		return "◑"
	default:
		return "○"
	}
}

func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}
