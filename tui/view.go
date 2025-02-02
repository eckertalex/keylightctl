package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	var s string

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))
	header := titleStyle.Render("KeyLight TUI - Full Screen")
	globalStatus := fmt.Sprintf("Global Power: %v (toggle with 'g')", m.GlobalOn)
	headerBlock := header + "\n" + globalStatus + "\n\n"

	var body string
	for i, light := range m.Lights {
		baseCardStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Margin(1)

		var dynamicStyle lipgloss.Style
		if light.On {
			dynamicStyle = baseCardStyle.BorderForeground(lipgloss.Color("2"))
		} else {
			dynamicStyle = baseCardStyle.BorderForeground(lipgloss.Color("240"))
		}

		onStatus := "OFF"
		if light.On {
			onStatus = "ON"
		}
		lightHeader := fmt.Sprintf("%s [%s]", light.Name, onStatus)
		brightnessRatio := float64(light.Brightness) / 100.0
		tempRatio := float64(light.Temperature-2900) / float64(7000-2900)
		brightnessBarStr := m.brightnessBar.ViewAs(brightnessRatio)
		temperatureBarStr := m.temperatureBar.ViewAs(tempRatio)
		brightnessText := fmt.Sprintf("Brightness: %d%%   %s", light.Brightness, brightnessBarStr)
		temperatureText := fmt.Sprintf("Temp: %dK   %s", light.Temperature, temperatureBarStr)
		bodyBlock := lipgloss.JoinVertical(lipgloss.Left, lightHeader, brightnessText, temperatureText)

		if i == m.Cursor {
			dynamicStyle = dynamicStyle.BorderStyle(lipgloss.ThickBorder())
			bodyBlock = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Render(bodyBlock)
		}

		body += dynamicStyle.Render(bodyBlock) + "\n"
	}

	s = headerBlock + body

	footer := "\nControls: ↑/k, ↓/j: Navigate | Enter: Toggle | g: Global toggle | r: Refresh | +: Brightness up, -: down | n: Temp up, m: Temp down | q: Quit"

	linesUsed := countLines(s) + countLines(footer)
	spacerLines := m.height - linesUsed
	if spacerLines < 0 {
		spacerLines = 0
	}
	spacer := ""
	for i := 0; i < spacerLines; i++ {
		spacer += "\n"
	}

	return s + spacer + footer
}

func countLines(s string) int {
	count := 0
	for _, r := range s {
		if r == '\n' {
			count++
		}
	}
	return count + 1
}
