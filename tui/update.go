package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "t":
			m.lightOn = !m.lightOn
		case "j":
			if m.brightness < 100 {
				m.brightness++
			}
		case "k":
			if m.brightness > 0 {
				m.brightness--
			}
		case "n":
			if m.colorTemp < 6500 {
				m.colorTemp += 100
			}
		case "m":
			if m.colorTemp > 2700 {
				m.colorTemp -= 100
			}
		}
	}
	return m, nil
}

