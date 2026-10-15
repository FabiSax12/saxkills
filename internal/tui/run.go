package tui

import tea "github.com/charmbracelet/bubbletea"

func Run(svc Services) error {
	_, err := tea.NewProgram(New(svc), tea.WithAltScreen()).Run()
	return err
}
