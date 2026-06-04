package main

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// --- config ---

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return path
}

func TestLoadConfig(t *testing.T) {
	path := writeConfig(t, `{"lights":[{"name":"Left","ip":"192.168.1.1:9123"}]}`)
	lights, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(lights) != 1 {
		t.Fatalf("len = %d, want 1", len(lights))
	}
	if lights[0].Name != "Left" || lights[0].IP != "192.168.1.1:9123" {
		t.Errorf("light = %+v, want {Left 192.168.1.1:9123}", lights[0])
	}
}

func TestLoadConfigMultipleLights(t *testing.T) {
	path := writeConfig(t, `{"lights":[
		{"name":"Left","ip":"192.168.1.1:9123"},
		{"name":"Right","ip":"192.168.1.2:9123"}
	]}`)
	lights, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(lights) != 2 {
		t.Fatalf("len = %d, want 2", len(lights))
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := loadConfig(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	path := writeConfig(t, `not json`)
	_, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadConfigEmptyLights(t *testing.T) {
	path := writeConfig(t, `{"lights":[]}`)
	lights, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(lights) != 0 {
		t.Errorf("len = %d, want 0", len(lights))
	}
}

// --- validation ---

func TestValidateBrightness(t *testing.T) {
	tests := []struct {
		n    int
		want bool
	}{
		{0, true},
		{50, true},
		{100, true},
		{-1, false},
		{101, false},
	}
	for _, tt := range tests {
		err := validateBrightness(tt.n)
		if (err == nil) != tt.want {
			t.Errorf("validateBrightness(%d): ok=%v, want %v", tt.n, err == nil, tt.want)
		}
	}
}

func TestValidateTemperature(t *testing.T) {
	tests := []struct {
		n    int
		want bool
	}{
		{2900, true},
		{5000, true},
		{7000, true},
		{2899, false},
		{7001, false},
	}
	for _, tt := range tests {
		err := validateTemperature(tt.n)
		if (err == nil) != tt.want {
			t.Errorf("validateTemperature(%d): ok=%v, want %v", tt.n, err == nil, tt.want)
		}
	}
}

// --- helpers ---

func TestFormatOnOff(t *testing.T) {
	if formatOnOff(1) != "ON" {
		t.Errorf("formatOnOff(1) = %q, want ON", formatOnOff(1))
	}
	if formatOnOff(0) != "OFF" {
		t.Errorf("formatOnOff(0) = %q, want OFF", formatOnOff(0))
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{context.DeadlineExceeded, "timeout while connecting"},
		{context.Canceled, "timeout while connecting"},
		{io.EOF, "connection closed unexpectedly"},
		{&net.OpError{Op: "dial"}, "failed to connect"},
	}
	for _, tt := range tests {
		if got := classifyError(tt.err); got != tt.want {
			t.Errorf("classifyError(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

// --- light resolution ---

func TestResolveLights(t *testing.T) {
	lights := []LightConfig{
		{Name: "Left", IP: "192.168.1.1:9123"},
		{Name: "Right", IP: "192.168.1.2:9123"},
	}

	t.Run("empty name returns all", func(t *testing.T) {
		got, ok := resolveLights(lights, "")
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
	})

	t.Run("known name returns one", func(t *testing.T) {
		got, ok := resolveLights(lights, "Left")
		if !ok {
			t.Fatal("ok = false, want true")
		}
		if len(got) != 1 || got[0].Name != "Left" {
			t.Errorf("got %+v, want [{Left ...}]", got)
		}
	})

	t.Run("unknown name returns false", func(t *testing.T) {
		_, ok := resolveLights(lights, "Middle")
		if ok {
			t.Fatal("ok = true, want false")
		}
	})
}
