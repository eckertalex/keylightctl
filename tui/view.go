package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

var (
	lightOnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	lightOffStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

func (m Model) View() string {
	var lightStatus string
	var lightStyle lipgloss.Style
	if m.lightOn {
		lightStatus = "On"
		lightStyle = lightOnStyle
	} else {
		lightStatus = "Off"
		lightStyle = lightOffStyle
	}

	return fmt.Sprintf(
		"\n%s\n\n%sLight Status: %s\nBrightness: %d%%\nColor Temperature: %dK\n\n%s[Press 'q' to quit, 't' to toggle, 'j' to increase brightness, 'k' to decrease brightness, 'n' to increase temperature, 'm' to decrease temperature]",
		lightStyle.Render("Elgato Key Light Control"),
		lightStyle.Render("Status: "), lightStatus, m.brightness, m.colorTemp,
		lightStyle.Render("Controls: "),
	)
}

