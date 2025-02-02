package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const cardWidth = 72

func baseCardStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(cardWidth)
}

func applySelection(style lipgloss.Style, selected bool) lipgloss.Style {
	if selected {
		return style.BorderForeground(lipgloss.Color("2"))
	}
	return style
}

func formatStatus(isOn bool) string {
	if isOn {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("ON")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("OFF")
}

func renderGlobalCard(globalOn bool) string {
	globalText := lipgloss.NewStyle().Bold(true).Render("Global Power: " + formatStatus(globalOn))
	card := baseCardStyle()
	return card.Render(globalText)
}

func renderLightCard(light Light, isSelected bool, brightnessBar, temperatureBar Bar) string {
	card := applySelection(baseCardStyle(), isSelected)

	lightHeader := lipgloss.NewStyle().Bold(true).Render(light.Name + " " + formatStatus(light.On))

	brightnessRatio := float64(light.Brightness) / 100.0
	tempRatio := float64(light.Temperature-2900) / float64(7000-2900)
	brightnessBarStr := brightnessBar.ViewAs(brightnessRatio)
	temperatureBarStr := temperatureBar.ViewAs(tempRatio)

	brightnessText := fmt.Sprintf("Brightness: %d%%  %s", light.Brightness, brightnessBarStr)
	temperatureText := fmt.Sprintf("Temp: %dK  %s", light.Temperature, temperatureBarStr)

	bodyBlock := lipgloss.JoinVertical(lipgloss.Left, lightHeader, brightnessText, temperatureText)

	if isSelected {
		bodyBlock = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Render(bodyBlock)
	}

	return card.Render(bodyBlock)
}

func renderFooter() string {
	footerStyle := baseCardStyle().
		BorderForeground(lipgloss.Color("240")).
		Foreground(lipgloss.Color("240"))

	controlsText := "↑/k, ↓/j: Move | Enter: Toggle | g: Toggle all | r: Refresh\n+/-: Brightness | n/m: Temperature | q: Quit"

	return footerStyle.Render(controlsText)
}

type Bar interface {
	ViewAs(ratio float64) string
}

func (m Model) View() string {
	globalCard := renderGlobalCard(m.GlobalOn)

	lightCards := make([]string, len(m.Lights))
	for i, light := range m.Lights {
		lightCards[i] = renderLightCard(light, i == m.Cursor, m.brightnessBar, m.temperatureBar)
	}

	footer := renderFooter()

	globalHeight := strings.Count(globalCard, "\n") + 1
	footerHeight := strings.Count(footer, "\n") + 1
	lightsHeight := 0
	for _, light := range lightCards {
		lightsHeight += strings.Count(light, "\n") + 1
	}
	spacerHeight := m.height - globalHeight - lightsHeight - footerHeight - 1
	if spacerHeight < 0 {
		spacerHeight = 0
	}

	spacer := strings.Repeat("\n", spacerHeight)
	content := append([]string{globalCard}, lightCards...)
	content = append(content, spacer)
	content = append(content, footer)

	return lipgloss.JoinVertical(lipgloss.Left, content...)
}
