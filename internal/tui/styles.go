package tui

import "github.com/charmbracelet/lipgloss"

var (
	colAccent = lipgloss.AdaptiveColor{Light: "#5A3FC0", Dark: "#B4A0FF"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#6B6B6B", Dark: "#8A8A8A"}
	colOK     = lipgloss.AdaptiveColor{Light: "#1E7B34", Dark: "#6BD08A"}
	colWarn   = lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#E3B341"}
	colErr    = lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#FF8A80"}

	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleMuted  = lipgloss.NewStyle().Foreground(colMuted)
	styleOK     = lipgloss.NewStyle().Foreground(colOK)
	styleWarn   = lipgloss.NewStyle().Foreground(colWarn)
	styleErr    = lipgloss.NewStyle().Foreground(colErr)
	styleCursor = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleBold   = lipgloss.NewStyle().Bold(true)
)
