package tui

import (
	"fmt"
	"net"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

func cardContentWidth(termWidth int) int {
	if termWidth > 76 {
		return termWidth - 4
	}
	return 72
}

func baseCardStyle(contentWidth int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(contentWidth)
}

func applySelection(style lipgloss.Style, selected bool) lipgloss.Style {
	if selected {
		return style.
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "2", Dark: "10"})
	}
	return style
}

func formatStatus(isOn bool) string {
	if isOn {
		return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "2", Dark: "10"}).Render("ON")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "1", Dark: "9"}).Render("OFF")
}

func renderGlobalCard(globalOn bool, contentWidth int) string {
	globalText := lipgloss.NewStyle().Bold(true).Render("Global Power: " + formatStatus(globalOn))
	return baseCardStyle(contentWidth).Render(globalText)
}

func lightHost(ip string) string {
	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		return ip
	}
	return host
}

func renderLightCard(light Light, isSelected bool, propertyCursor int, brightnessBar, temperatureBar progress.Model, contentWidth int) string {
	card := applySelection(baseCardStyle(contentWidth), isSelected)

	var nameStr string
	if isSelected {
		nameStr = "[" + light.Name + "]"
	} else {
		nameStr = " " + light.Name + " "
	}
	ipStr := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "240"}).Render(lightHost(light.IP))
	lightHeader := lipgloss.NewStyle().Bold(true).Render(nameStr+"  "+formatStatus(light.On)) + "  " + ipStr

	brightnessPrefix := "  "
	temperaturePrefix := "  "
	if isSelected {
		if propertyCursor == 0 {
			brightnessPrefix = "▶ "
		} else {
			temperaturePrefix = "▶ "
		}
	}

	brightnessRatio := float64(light.Brightness) / 100.0
	tempRatio := float64(light.Temperature-2900) / float64(7000-2900)

	brightnessText := fmt.Sprintf("%sBrightness   %3d%%  %s", brightnessPrefix, light.Brightness, brightnessBar.ViewAs(brightnessRatio))
	temperatureText := fmt.Sprintf("%sTemperature %4dK  %s", temperaturePrefix, light.Temperature, temperatureBar.ViewAs(tempRatio))

	bodyBlock := lipgloss.JoinVertical(lipgloss.Left, lightHeader, brightnessText, temperatureText)
	return card.Render(bodyBlock)
}

func renderFooter(termWidth int) string {
	text := "  h/l light  ·  j/k property  ·  =/- adjust  ·  enter toggle  ·  a all  ·  r refresh  ·  q quit"
	if termWidth > 0 && termWidth < 100 {
		text = "  h/l  ·  j/k  ·  =/- adjust  ·  enter  ·  a  ·  r  ·  q"
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "240"}).
		Render(text)
}

func (m Model) View() string {
	cw := cardContentWidth(m.width)
	globalCard := renderGlobalCard(m.GlobalOn, cw)

	lightCards := make([]string, len(m.Lights))
	for i, light := range m.Lights {
		lightCards[i] = renderLightCard(light, i == m.Cursor, m.PropertyCursor, m.brightnessBar, m.temperatureBar, cw)
	}

	footer := renderFooter(m.width)

	var statusLine string
	if m.err != nil {
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "1", Dark: "9"}).Render("Error: " + m.err.Error())
	} else if m.status != "" {
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "240"}).Render(m.status)
	}

	globalHeight := strings.Count(globalCard, "\n") + 1
	footerHeight := strings.Count(footer, "\n") + 1
	lightsHeight := 0
	for _, lc := range lightCards {
		lightsHeight += strings.Count(lc, "\n") + 1
	}
	spacerHeight := max(m.height-globalHeight-lightsHeight-footerHeight-2, 0)

	spacer := strings.Repeat("\n", spacerHeight)
	content := append([]string{globalCard}, lightCards...)
	content = append(content, spacer, footer, statusLine)

	return lipgloss.JoinVertical(lipgloss.Left, content...)
}
