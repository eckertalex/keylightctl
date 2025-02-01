package tui

import tea "github.com/charmbracelet/bubbletea"

type Model struct {
	lightOn    bool
	brightness int
	colorTemp  int
}

func (m Model) Init() tea.Cmd {
	return nil
}

func initialModel() Model {
	return Model{
		lightOn:    false,
		brightness: 50,
		colorTemp:  4000,
	}
}
