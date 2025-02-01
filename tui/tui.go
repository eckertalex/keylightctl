package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/eckertalex/keylightctl/internal/keylight"
)

func Run(lightsConfig []keylight.LightConfig) error {
	p := tea.NewProgram(initialModel(lightsConfig), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
