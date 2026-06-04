package main

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func runTUI(lights []LightConfig) {
	p := tea.NewProgram(newTUIModel(lights), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running TUI:", err)
	}
}

// --- model ---

type tuiLight struct {
	Name        string
	IP          string
	On          bool
	Brightness  int
	Temperature int
}

type tuiModel struct {
	GlobalOn       bool
	Lights         []tuiLight
	Cursor         int
	PropertyCursor int // 0 = brightness, 1 = temperature

	brightnessBar  progress.Model
	temperatureBar progress.Model

	err             error
	status          string
	pendingRequests int
	width, height   int
}

func newTUIModel(configs []LightConfig) tuiModel {
	pb := progress.New(progress.WithDefaultGradient())
	lights := make([]tuiLight, len(configs))
	for i, cfg := range configs {
		lights[i] = tuiLight{Name: cfg.Name, IP: cfg.IP, Brightness: 20, Temperature: 5000}
	}
	return tuiModel{
		Lights:          lights,
		brightnessBar:   pb,
		temperatureBar:  pb,
		status:          "Loading...",
		pendingRequests: len(configs),
	}
}

func (m tuiModel) Init() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.Lights))
	for i, l := range m.Lights {
		cmds[i] = cmdFetchStatus(i, l.IP)
	}
	return tea.Batch(cmds...)
}

func (m tuiModel) currentSettings(idx int) LightDetail {
	l := m.Lights[idx]
	on := 0
	if l.On {
		on = 1
	}
	return LightDetail{On: on, Brightness: l.Brightness, Temperature: kelvinToMired(l.Temperature)}
}

// --- messages ---

type msgLightStatus struct {
	index  int
	detail LightDetail
	err    error
}

type msgLightUpdate struct {
	index  int
	detail LightDetail
	err    error
}

func cmdFetchStatus(index int, ip string) tea.Cmd {
	return func() tea.Msg {
		c := newController()
		status, err := c.getLight(ip)
		var detail LightDetail
		if err == nil && len(status.Lights) > 0 {
			detail = status.Lights[0]
		} else if err == nil {
			err = errors.New("empty status")
		}
		return msgLightStatus{index: index, detail: detail, err: err}
	}
}

func cmdUpdateLight(index int, ip string, settings LightDetail) tea.Cmd {
	return func() tea.Msg {
		c := newController()
		status, err := c.updateLight(ip, settings)
		var detail LightDetail
		if err == nil && len(status.Lights) > 0 {
			detail = status.Lights[0]
		} else if err == nil {
			err = errors.New("empty status")
		}
		return msgLightUpdate{index: index, detail: detail, err: err}
	}
}

// --- update ---

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "r", "R":
			m.pendingRequests += len(m.Lights)
			m.status = "Refreshing..."
			cmds := make([]tea.Cmd, len(m.Lights))
			for i := range m.Lights {
				cmds[i] = cmdFetchStatus(i, m.Lights[i].IP)
			}
			return m, tea.Batch(cmds...)
		case "a", "A":
			m.GlobalOn = !m.GlobalOn
			m.pendingRequests += len(m.Lights)
			m.status = "Toggling all lights..."
			cmds := make([]tea.Cmd, len(m.Lights))
			for i := range m.Lights {
				m.Lights[i].On = m.GlobalOn
				cmds[i] = cmdUpdateLight(i, m.Lights[i].IP, m.currentSettings(i))
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
			return m, cmdUpdateLight(idx, m.Lights[idx].IP, m.currentSettings(idx))
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
			return m, cmdUpdateLight(idx, m.Lights[idx].IP, m.currentSettings(idx))
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
			return m, cmdUpdateLight(idx, m.Lights[idx].IP, m.currentSettings(idx))
		}
	case msgLightStatus:
		m.pendingRequests = max(0, m.pendingRequests-1)
		if msg.err != nil {
			m.err = fmt.Errorf("%s: %w", m.Lights[msg.index].Name, msg.err)
			break
		}
		m.err = nil
		m.Lights[msg.index].On = msg.detail.On == 1
		m.Lights[msg.index].Brightness = msg.detail.Brightness
		m.Lights[msg.index].Temperature = miredToKelvin(msg.detail.Temperature)
		m.GlobalOn = !slices.ContainsFunc(m.Lights, func(l tuiLight) bool { return !l.On })
		if m.pendingRequests == 0 {
			m.status = ""
		}
	case msgLightUpdate:
		m.pendingRequests = max(0, m.pendingRequests-1)
		if msg.err != nil {
			m.err = fmt.Errorf("%s: %w", m.Lights[msg.index].Name, msg.err)
			break
		}
		m.err = nil
		m.Lights[msg.index].On = msg.detail.On == 1
		m.Lights[msg.index].Brightness = msg.detail.Brightness
		m.Lights[msg.index].Temperature = miredToKelvin(msg.detail.Temperature)
		m.GlobalOn = !slices.ContainsFunc(m.Lights, func(l tuiLight) bool { return !l.On })
		if m.pendingRequests == 0 {
			m.status = ""
		}
	}
	return m, nil
}

// --- view ---

func (m tuiModel) View() string {
	cw := cardContentWidth(m.width)
	cards := []string{renderGlobalCard(m.GlobalOn, cw)}
	for i, l := range m.Lights {
		cards = append(cards, renderLightCard(l, i == m.Cursor, m.PropertyCursor, m.brightnessBar, m.temperatureBar, cw))
	}
	footer := renderFooter(m.width)

	var statusLine string
	if m.err != nil {
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "1", Dark: "9"}).Render("Error: " + m.err.Error())
	} else if m.status != "" {
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "240"}).Render(m.status)
	}

	globalH := strings.Count(cards[0], "\n") + 1
	footerH := strings.Count(footer, "\n") + 1
	lightsH := 0
	for _, c := range cards[1:] {
		lightsH += strings.Count(c, "\n") + 1
	}
	spacer := strings.Repeat("\n", max(m.height-globalH-lightsH-footerH-2, 0))

	return lipgloss.JoinVertical(lipgloss.Left, append(cards, spacer, footer, statusLine)...)
}

func cardContentWidth(termWidth int) int {
	if termWidth > 76 {
		return termWidth - 4
	}
	return 72
}

func baseCardStyle(w int) lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(w)
}

func renderGlobalCard(globalOn bool, cw int) string {
	return baseCardStyle(cw).Render(lipgloss.NewStyle().Bold(true).Render("Global Power: " + statusColor(globalOn)))
}

func renderLightCard(l tuiLight, selected bool, propCursor int, bb, tb progress.Model, cw int) string {
	card := baseCardStyle(cw)
	if selected {
		card = card.BorderStyle(lipgloss.ThickBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "2", Dark: "10"})
	}

	name := " " + l.Name + " "
	if selected {
		name = "[" + l.Name + "]"
	}
	ip := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "240"}).Render(lightHost(l.IP))
	header := lipgloss.NewStyle().Bold(true).Render(name+"  "+statusColor(l.On)) + "  " + ip

	bPrefix, tPrefix := "  ", "  "
	if selected {
		if propCursor == 0 {
			bPrefix = "▶ "
		} else {
			tPrefix = "▶ "
		}
	}

	bLine := fmt.Sprintf("%sBrightness   %3d%%  %s", bPrefix, l.Brightness, bb.ViewAs(float64(l.Brightness)/100.0))
	tLine := fmt.Sprintf("%sTemperature %4dK  %s", tPrefix, l.Temperature, tb.ViewAs(float64(l.Temperature-2900)/float64(7000-2900)))

	return card.Render(lipgloss.JoinVertical(lipgloss.Left, header, bLine, tLine))
}

func renderFooter(termWidth int) string {
	text := "  h/l light  ·  j/k property  ·  =/- adjust  ·  enter toggle  ·  a all  ·  r refresh  ·  q quit"
	if termWidth > 0 && termWidth < 100 {
		text = "  h/l  ·  j/k  ·  =/- adjust  ·  enter  ·  a  ·  r  ·  q"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "240"}).Render(text)
}

func statusColor(on bool) string {
	if on {
		return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "2", Dark: "10"}).Render("ON")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "1", Dark: "9"}).Render("OFF")
}

func lightHost(ip string) string {
	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		return ip
	}
	return host
}
