package tui

import (
	"fmt"
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
			m.pendingRequests += len(m.Lights)
			m.status = "Refreshing..."
			var cmds []tea.Cmd
			for i := range m.Lights {
				cmds = append(cmds, fetchLightStatus(i, m.Lights[i].IP))
			}
			return m, tea.Batch(cmds...)
		case "a", "A":
			m.GlobalOn = !m.GlobalOn
			m.pendingRequests += len(m.Lights)
			m.status = "Toggling all lights..."
			var cmds []tea.Cmd
			for i := range m.Lights {
				m.Lights[i].On = m.GlobalOn
				cmds = append(cmds, updateLight(i, m.Lights[i].IP, m.currentSettings(i)))
			}
			return m, tea.Batch(cmds...)
		case "h", "left":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "l", "right":
			if m.Cursor < len(m.Lights)-1 {
				m.Cursor++
			}
		case "j", "down", "tab":
			m.PropertyCursor = (m.PropertyCursor + 1) % 2
		case "k", "up", "shift+tab":
			m.PropertyCursor = (m.PropertyCursor - 1 + 2) % 2
		case "enter":
			idx := m.Cursor
			m.Lights[idx].On = !m.Lights[idx].On
			m.pendingRequests++
			m.status = fmt.Sprintf("Toggling %s...", m.Lights[idx].Name)
			return m, updateLight(idx, m.Lights[idx].IP, m.currentSettings(idx))
		case "=":
			idx := m.Cursor
			switch m.PropertyCursor {
			case 0:
				m.Lights[idx].Brightness = min(m.Lights[idx].Brightness+5, 100)
			case 1:
				m.Lights[idx].Temperature = min(m.Lights[idx].Temperature+100, 7000)
			}
			m.pendingRequests++
			m.status = fmt.Sprintf("Updating %s...", m.Lights[idx].Name)
			return m, updateLight(idx, m.Lights[idx].IP, m.currentSettings(idx))
		case "-":
			idx := m.Cursor
			switch m.PropertyCursor {
			case 0:
				m.Lights[idx].Brightness = max(m.Lights[idx].Brightness-5, 0)
			case 1:
				m.Lights[idx].Temperature = max(m.Lights[idx].Temperature-100, 2900)
			}
			m.pendingRequests++
			m.status = fmt.Sprintf("Updating %s...", m.Lights[idx].Name)
			return m, updateLight(idx, m.Lights[idx].IP, m.currentSettings(idx))
		}
	case lightStatusMsg:
		m.pendingRequests = max(0, m.pendingRequests-1)
		if msg.err != nil {
			m.err = fmt.Errorf("%s: %w", m.Lights[msg.index].Name, msg.err)
			break
		}
		m.err = nil

		m.Lights[msg.index].On = msg.status.On == 1
		m.Lights[msg.index].Brightness = msg.status.Brightness
		m.Lights[msg.index].Temperature = keylight.MiredToKelvin(msg.status.Temperature)

		m.GlobalOn = !slices.ContainsFunc(m.Lights, func(l Light) bool {
			return !l.On
		})
		if m.pendingRequests == 0 {
			m.status = ""
		}
	case lightUpdateMsg:
		m.pendingRequests = max(0, m.pendingRequests-1)
		if msg.err != nil {
			m.err = fmt.Errorf("%s: %w", m.Lights[msg.index].Name, msg.err)
			break
		}
		m.err = nil

		m.Lights[msg.index].On = msg.status.On == 1
		m.Lights[msg.index].Brightness = msg.status.Brightness
		m.Lights[msg.index].Temperature = keylight.MiredToKelvin(msg.status.Temperature)

		m.GlobalOn = !slices.ContainsFunc(m.Lights, func(l Light) bool {
			return !l.On
		})
		if m.pendingRequests == 0 {
			m.status = ""
		}
	}

	return m, nil
}
