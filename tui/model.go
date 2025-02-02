package tui

import (
	"errors"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/eckertalex/keylightctl/internal/keylight"
)

type Light struct {
	Name        string
	IP          string
	On          bool
	Brightness  int
	Temperature int
}

type Model struct {
	GlobalOn bool

	Lights []Light

	Cursor int

	brightnessBar  progress.Model
	temperatureBar progress.Model

	width  int
	height int
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for i, light := range m.Lights {
		cmds = append(cmds, fetchLightStatus(i, light.IP))
	}
	return tea.Batch(cmds...)
}

func NewModel(configs []keylight.LightConfig) Model {
	pb := progress.New(progress.WithDefaultGradient())
	lights := make([]Light, len(configs))

	for i, cfg := range configs {
		lights[i] = Light{
			Name:        cfg.Name,
			IP:          cfg.IP,
			On:          false,
			Brightness:  20,
			Temperature: 5000,
		}
	}
	return Model{
		GlobalOn:       false,
		Lights:         lights,
		Cursor:         0,
		brightnessBar:  pb,
		temperatureBar: pb,
	}
}

func initialModel(configs []keylight.LightConfig) Model {
	return NewModel(configs)
}

type lightStatusMsg struct {
	index  int
	status keylight.LightDetail
	err    error
}

type lightUpdateMsg struct {
	index  int
	status keylight.LightDetail
	err    error
}

func fetchLightStatus(index int, ip string) tea.Cmd {
	return func() tea.Msg {
		controller := keylight.NewController()
		status, err := controller.GetLight(ip)
		var detail keylight.LightDetail
		if err == nil && len(status.Lights) > 0 {
			detail = status.Lights[0]
		} else if err == nil {
			err = errors.New("empty status")
		}
		return lightStatusMsg{index: index, status: detail, err: err}
	}
}

func updateLight(index int, ip string, settings keylight.LightDetail) tea.Cmd {
	return func() tea.Msg {
		controller := keylight.NewController()
		status, err := controller.UpdateLight(ip, settings)
		var detail keylight.LightDetail
		if err == nil && len(status.Lights) > 0 {
			detail = status.Lights[0]
		} else if err == nil {
			err = errors.New("empty update status")
		}
		return lightUpdateMsg{index: index, status: detail, err: err}
	}
}
