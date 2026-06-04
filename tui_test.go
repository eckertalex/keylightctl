package main

import (
	"errors"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func makeTUIModel(n int) tuiModel {
	configs := make([]LightConfig, n)
	for i := range configs {
		configs[i] = LightConfig{
			Name: fmt.Sprintf("Light%d", i+1),
			IP:   fmt.Sprintf("192.168.1.%d:9123", i+1),
		}
	}
	return newTUIModel(configs)
}

func press(m tuiModel, key string) tuiModel {
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		msg = tea.KeyMsg{Type: tea.KeyShiftTab}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	next, _ := m.Update(msg)
	return next.(tuiModel)
}

// --- init ---

func TestNewTUIModel(t *testing.T) {
	m := makeTUIModel(2)
	if len(m.Lights) != 2 {
		t.Fatalf("len(Lights) = %d, want 2", len(m.Lights))
	}
	if m.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0", m.Cursor)
	}
	if m.PropertyCursor != 0 {
		t.Errorf("PropertyCursor = %d, want 0", m.PropertyCursor)
	}
	if m.pendingRequests != 2 {
		t.Errorf("pendingRequests = %d, want 2", m.pendingRequests)
	}
	if m.status != "Loading..." {
		t.Errorf("status = %q, want %q", m.status, "Loading...")
	}
}

// --- navigation ---

func TestTUICursorNavigation(t *testing.T) {
	m := makeTUIModel(3)

	m = press(m, "l")
	if m.Cursor != 1 {
		t.Errorf("after l: Cursor = %d, want 1", m.Cursor)
	}
	m = press(m, "right")
	if m.Cursor != 2 {
		t.Errorf("after right: Cursor = %d, want 2", m.Cursor)
	}
	m = press(m, "l") // at end
	if m.Cursor != 2 {
		t.Errorf("at end after l: Cursor = %d, want 2", m.Cursor)
	}
	m = press(m, "h")
	if m.Cursor != 1 {
		t.Errorf("after h: Cursor = %d, want 1", m.Cursor)
	}
	m = press(m, "left")
	m = press(m, "h") // at start
	if m.Cursor != 0 {
		t.Errorf("at start after h: Cursor = %d, want 0", m.Cursor)
	}
}

func TestTUIPropertyCursor(t *testing.T) {
	m := makeTUIModel(1)

	m = press(m, "j")
	if m.PropertyCursor != 1 {
		t.Errorf("after j: PropertyCursor = %d, want 1", m.PropertyCursor)
	}
	m = press(m, "j") // wraps to 0
	if m.PropertyCursor != 0 {
		t.Errorf("after j wrap: PropertyCursor = %d, want 0", m.PropertyCursor)
	}
	m = press(m, "k") // wraps to 1
	if m.PropertyCursor != 1 {
		t.Errorf("after k wrap: PropertyCursor = %d, want 1", m.PropertyCursor)
	}
	m = press(m, "k")
	if m.PropertyCursor != 0 {
		t.Errorf("after k: PropertyCursor = %d, want 0", m.PropertyCursor)
	}
	m = press(m, "tab")
	if m.PropertyCursor != 1 {
		t.Errorf("after tab: PropertyCursor = %d, want 1", m.PropertyCursor)
	}
	m = press(m, "shift+tab")
	if m.PropertyCursor != 0 {
		t.Errorf("after shift+tab: PropertyCursor = %d, want 0", m.PropertyCursor)
	}
}

// --- brightness ---

func TestTUIBrightnessAdjust(t *testing.T) {
	m := makeTUIModel(1)
	m.Lights[0].Brightness = 50
	m.PropertyCursor = 0

	m = press(m, "=")
	if m.Lights[0].Brightness != 55 {
		t.Errorf("after =: Brightness = %d, want 55", m.Lights[0].Brightness)
	}
	m = press(m, "-")
	if m.Lights[0].Brightness != 50 {
		t.Errorf("after -: Brightness = %d, want 50", m.Lights[0].Brightness)
	}

	m.Lights[0].Brightness = 98
	m = press(m, "=")
	if m.Lights[0].Brightness != 100 {
		t.Errorf("max clamp: Brightness = %d, want 100", m.Lights[0].Brightness)
	}
	m = press(m, "=") // already at max
	if m.Lights[0].Brightness != 100 {
		t.Errorf("stays at max: Brightness = %d, want 100", m.Lights[0].Brightness)
	}

	m.Lights[0].Brightness = 2
	m = press(m, "-")
	if m.Lights[0].Brightness != 0 {
		t.Errorf("min clamp: Brightness = %d, want 0", m.Lights[0].Brightness)
	}
	m = press(m, "-") // already at min
	if m.Lights[0].Brightness != 0 {
		t.Errorf("stays at min: Brightness = %d, want 0", m.Lights[0].Brightness)
	}
}

// --- temperature ---

func TestTUITemperatureAdjust(t *testing.T) {
	m := makeTUIModel(1)
	m.Lights[0].Temperature = 5000
	m.PropertyCursor = 1

	m = press(m, "=")
	if m.Lights[0].Temperature != 5100 {
		t.Errorf("after =: Temperature = %d, want 5100", m.Lights[0].Temperature)
	}
	m = press(m, "-")
	if m.Lights[0].Temperature != 5000 {
		t.Errorf("after -: Temperature = %d, want 5000", m.Lights[0].Temperature)
	}

	m.Lights[0].Temperature = 6950
	m = press(m, "=")
	if m.Lights[0].Temperature != 7000 {
		t.Errorf("max clamp: Temperature = %d, want 7000", m.Lights[0].Temperature)
	}

	m.Lights[0].Temperature = 2950
	m = press(m, "-")
	if m.Lights[0].Temperature != 2900 {
		t.Errorf("min clamp: Temperature = %d, want 2900", m.Lights[0].Temperature)
	}
}

// --- toggle ---

func TestTUIToggleLight(t *testing.T) {
	m := makeTUIModel(1)
	m.Lights[0].On = false

	m = press(m, "enter")
	if !m.Lights[0].On {
		t.Error("after enter: light should be ON")
	}
	m = press(m, "enter")
	if m.Lights[0].On {
		t.Error("after second enter: light should be OFF")
	}
}

func TestTUIToggleAll(t *testing.T) {
	m := makeTUIModel(2)
	m.Lights[0].On = false
	m.Lights[1].On = false
	m.GlobalOn = false

	m = press(m, "a")
	if !m.GlobalOn {
		t.Error("after a: GlobalOn should be true")
	}
	if !m.Lights[0].On || !m.Lights[1].On {
		t.Error("after a: all lights should be ON")
	}

	m = press(m, "a")
	if m.GlobalOn {
		t.Error("after second a: GlobalOn should be false")
	}
	if m.Lights[0].On || m.Lights[1].On {
		t.Error("after second a: all lights should be OFF")
	}
}

// --- messages ---

func TestTUIMsgLightStatus(t *testing.T) {
	m := makeTUIModel(2)
	m.pendingRequests = 2

	next, _ := m.Update(msgLightStatus{
		index:  0,
		detail: LightDetail{On: 1, Brightness: 75, Temperature: kelvinToMired(4000)},
	})
	m = next.(tuiModel)

	if !m.Lights[0].On {
		t.Error("light 0 should be ON")
	}
	if m.Lights[0].Brightness != 75 {
		t.Errorf("Brightness = %d, want 75", m.Lights[0].Brightness)
	}
	if m.Lights[0].Temperature != 4000 {
		t.Errorf("Temperature = %d, want 4000", m.Lights[0].Temperature)
	}
	if m.pendingRequests != 1 {
		t.Errorf("pendingRequests = %d, want 1", m.pendingRequests)
	}
	if m.status == "" {
		t.Error("status should be non-empty while one request is still pending")
	}

	next, _ = m.Update(msgLightStatus{
		index:  1,
		detail: LightDetail{On: 1, Brightness: 60, Temperature: kelvinToMired(5000)},
	})
	m = next.(tuiModel)

	if m.pendingRequests != 0 {
		t.Errorf("pendingRequests = %d, want 0", m.pendingRequests)
	}
	if m.status != "" {
		t.Errorf("status should be empty when all resolved, got %q", m.status)
	}
	if !m.GlobalOn {
		t.Error("GlobalOn should be true when all lights are ON")
	}
}

func TestTUIMsgLightStatusError(t *testing.T) {
	m := makeTUIModel(1)
	m.pendingRequests = 1

	next, _ := m.Update(msgLightStatus{index: 0, err: errors.New("connection refused")})
	m = next.(tuiModel)

	if m.err == nil {
		t.Error("model.err should be set after error message")
	}
}

func TestTUIMsgLightUpdate(t *testing.T) {
	m := makeTUIModel(1)
	m.pendingRequests = 1

	next, _ := m.Update(msgLightUpdate{
		index:  0,
		detail: LightDetail{On: 0, Brightness: 30, Temperature: kelvinToMired(3000)},
	})
	m = next.(tuiModel)

	if m.Lights[0].On {
		t.Error("light should be OFF")
	}
	if m.Lights[0].Brightness != 30 {
		t.Errorf("Brightness = %d, want 30", m.Lights[0].Brightness)
	}
	if m.Lights[0].Temperature != 3000 {
		t.Errorf("Temperature = %d, want 3000", m.Lights[0].Temperature)
	}
	if m.GlobalOn {
		t.Error("GlobalOn should be false when light is OFF")
	}
}

// --- currentSettings ---

func TestTUICurrentSettings(t *testing.T) {
	m := makeTUIModel(1)
	m.Lights[0] = tuiLight{Name: "Test", IP: "192.168.1.1:9123", On: true, Brightness: 60, Temperature: 4000}

	s := m.currentSettings(0)
	if s.On != 1 {
		t.Errorf("On = %d, want 1", s.On)
	}
	if s.Brightness != 60 {
		t.Errorf("Brightness = %d, want 60", s.Brightness)
	}
	if s.Temperature != kelvinToMired(4000) {
		t.Errorf("Temperature = %d, want %d", s.Temperature, kelvinToMired(4000))
	}

	m.Lights[0].On = false
	s = m.currentSettings(0)
	if s.On != 0 {
		t.Errorf("off: On = %d, want 0", s.On)
	}
}

// --- helpers ---

func TestLightHost(t *testing.T) {
	tests := []struct{ in, want string }{
		{"192.168.1.1:9123", "192.168.1.1"},
		{"[::1]:8080", "::1"},
		{"notahost", "notahost"},
	}
	for _, tt := range tests {
		if got := lightHost(tt.in); got != tt.want {
			t.Errorf("lightHost(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCardContentWidth(t *testing.T) {
	tests := []struct{ in, want int }{
		{80, 76},
		{77, 73},
		{76, 72},
		{40, 72},
		{0, 72},
	}
	for _, tt := range tests {
		if got := cardContentWidth(tt.in); got != tt.want {
			t.Errorf("cardContentWidth(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
