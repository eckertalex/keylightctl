package tui

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eckertalex/keylightctl/internal/keylight"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "r", "R":
			var cmds []tea.Cmd
			for i := range m.Lights {
				m.Lights[i].On = m.GlobalOn
				cmds = append(cmds, fetchLightStatus(i, m.Lights[i].IP))
			}
			return m, tea.Batch(cmds...)
		case "g", "G":
			m.GlobalOn = !m.GlobalOn
			var cmds []tea.Cmd
			for i := range m.Lights {
				m.Lights[i].On = m.GlobalOn
				settings := keylight.LightDetail{
					On: func() int {
						if m.GlobalOn {
							return 1
						}
						return 0
					}(),
					Brightness:  m.Lights[i].Brightness,
					Temperature: keylight.KelvinToMired(m.Lights[i].Temperature),
				}
				cmds = append(cmds, updateLight(i, m.Lights[i].IP, settings))
			}
			return m, tea.Batch(cmds...)
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Lights)-1 {
				m.Cursor++
			}
		case "enter":
			idx := m.Cursor
			m.Lights[idx].On = !m.Lights[idx].On
			settings := keylight.LightDetail{
				On: func() int {
					if m.Lights[idx].On {
						return 1
					}
					return 0
				}(),
				Brightness:  m.Lights[idx].Brightness,
				Temperature: keylight.KelvinToMired(m.Lights[idx].Temperature),
			}
			return m, updateLight(idx, m.Lights[idx].IP, settings)
		case "+":
			idx := m.Cursor
			if m.Lights[idx].Brightness < 100 {
				m.Lights[idx].Brightness += 5
			}
			settings := keylight.LightDetail{
				On: func() int {
					if m.Lights[idx].On {
						return 1
					}
					return 0
				}(),
				Brightness:  m.Lights[idx].Brightness,
				Temperature: keylight.KelvinToMired(m.Lights[idx].Temperature),
			}
			return m, updateLight(idx, m.Lights[idx].IP, settings)
		case "-":
			idx := m.Cursor
			if m.Lights[idx].Brightness > 0 {
				m.Lights[idx].Brightness -= 5
			}
			settings := keylight.LightDetail{
				On: func() int {
					if m.Lights[idx].On {
						return 1
					}
					return 0
				}(),
				Brightness:  m.Lights[idx].Brightness,
				Temperature: keylight.KelvinToMired(m.Lights[idx].Temperature),
			}
			return m, updateLight(idx, m.Lights[idx].IP, settings)
		case "n":
			idx := m.Cursor
			if m.Lights[idx].Temperature < 7000 {
				m.Lights[idx].Temperature += 100
			}
			settings := keylight.LightDetail{
				On: func() int {
					if m.Lights[idx].On {
						return 1
					}
					return 0
				}(),
				Brightness:  m.Lights[idx].Brightness,
				Temperature: keylight.KelvinToMired(m.Lights[idx].Temperature),
			}
			return m, updateLight(idx, m.Lights[idx].IP, settings)
		case "m":
			idx := m.Cursor
			if m.Lights[idx].Temperature > 2900 {
				m.Lights[idx].Temperature -= 100
			}
			settings := keylight.LightDetail{
				On: func() int {
					if m.Lights[idx].On {
						return 1
					}
					return 0
				}(),
				Brightness:  m.Lights[idx].Brightness,
				Temperature: keylight.KelvinToMired(m.Lights[idx].Temperature),
			}
			return m, updateLight(idx, m.Lights[idx].IP, settings)
		}
	case lightStatusMsg:
		if msg.err != nil {
			// TODO: handle error
			break
		}

		m.Lights[msg.index].On = msg.status.On == 1
		m.Lights[msg.index].Brightness = msg.status.Brightness
		m.Lights[msg.index].Temperature = keylight.MiredToKelvin(msg.status.Temperature)

		m.GlobalOn = !slices.ContainsFunc(m.Lights, func(l Light) bool {
			return !l.On
		})
	case lightUpdateMsg:
		if msg.err != nil {
			// TODO: handle error
			break
		}

		m.Lights[msg.index].On = msg.status.On == 1
		m.Lights[msg.index].Brightness = msg.status.Brightness
		m.Lights[msg.index].Temperature = keylight.MiredToKelvin(msg.status.Temperature)

		m.GlobalOn = !slices.ContainsFunc(m.Lights, func(l Light) bool {
			return !l.On
		})
	}

	return m, nil
}
