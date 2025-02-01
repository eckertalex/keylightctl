package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type LightDetail struct {
	On          int `json:"on"`
	Brightness  int `json:"brightness,omitempty"`
	Temperature int `json:"temperature,omitempty"`
}

type LightStatus struct {
	Lights         []LightDetail `json:"lights,omitempty"`
	NumberOfLights int           `json:"numberOfLights,omitempty"`
}

var httpClient = &http.Client{
	Timeout: 3 * time.Second,
}

func getLightsURL(ip string) string {
	return fmt.Sprintf("http://%s/elgato/lights", ip)
}

func GetLightSettings(ip string) (*LightStatus, error) {
	resp, err := httpClient.Get(getLightsURL(ip))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response failed: %w", err)
	}

	var status LightStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("parsing response failed: %w", err)
	}

	return &status, nil
}

func UpdateLightSettings(ip string, settings LightDetail) (*LightStatus, error) {
	payload := LightStatus{
		Lights: []LightDetail{settings},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, getLightsURL(ip), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}
	if err != nil {
		return nil, fmt.Errorf("reading response failed: %w", err)
	}

	var status LightStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("parsing response failed: %w", err)
	}

	return &status, nil
}

func Spinner(done <-chan struct{}) {
	frames := []string{"|", "/", "-", "\\"}
	i := 0
	for {
		select {
		case <-done:
			fmt.Print("\r")
			return
		default:
			fmt.Printf("\r%s", frames[i%len(frames)])
			time.Sleep(100 * time.Millisecond)
			i++
		}
	}
}

func MiredToKelvin(mired int) int {
	// Mired is defined as 1 million divided by color temperature in Kelvin
	// So to get Kelvin from mired: K = 1000000/mired
	return roundToNearest50(1000000 / mired)
}

func KelvinToMired(kelvin int) int {
	// Mired is defined as 1,000,000 / Kelvin
	return 1000000 / kelvin
}

func roundToNearest50(n int) int {
	return (n + 25) / 50 * 50
}

func ValidateBrightness(brightness int) error {
	if brightness < 0 || brightness > 100 {
		return fmt.Errorf("brightness must be between 0 and 100")
	}
	return nil
}

func ValidateTemperature(temperature int) error {
	if temperature < 2900 || temperature > 7000 {
		return fmt.Errorf("temperature must be between 2900K and 7000K")
	}
	return nil
}

type Light struct {
	Name string `mapstructure:"name"`
	IP   string `mapstructure:"ip"`
}

type LightConfig struct {
	Light `mapstructure:",squash"`
}

type lightOperation func(ip string) (*LightStatus, error)

func processLightOperation(lights []Light, operation lightOperation, operationName string) {
	var wg sync.WaitGroup
	results := make(chan struct {
		err    error
		status *LightStatus
		name   string
	}, len(lights))

	done := make(chan struct{})
	go Spinner(done)

	for _, light := range lights {
		wg.Add(1)
		go func(light Light) {
			defer wg.Done()
			status, err := operation(light.IP)
			results <- struct {
				err    error
				status *LightStatus
				name   string
			}{err, status, light.Name}
		}(light)
	}

	go func() {
		wg.Wait()
		close(results)
		close(done)
	}()

	for result := range results {
		if result.err != nil {
			msg := "unknown error"
			switch {
			case errors.Is(result.err, context.DeadlineExceeded) ||
				errors.Is(result.err, context.Canceled):
				msg = "timeout while connecting"
			case errors.Is(result.err, io.EOF):
				msg = "connection closed unexpectedly"
			case isConnectionError(result.err):
				msg = "failed to connect"
			}
			fmt.Printf("\r%s of light \"%s\": Error: %s\n", operationName, result.name, msg)
			continue
		}

		for _, light := range result.status.Lights {
			fmt.Printf("\rStatus of light \"%s\":\n", result.name)
			fmt.Printf("  Power: %s\n", formatOnOff(light.On))
			fmt.Printf("  Brightness: %d%%\n", light.Brightness)
			fmt.Printf("  Temperature: %dK (mired: %d)\n",
				MiredToKelvin(light.Temperature),
				light.Temperature)
		}
	}
}

func isConnectionError(err error) bool {
	var netErr *net.OpError
	return errors.As(err, &netErr)
}

func formatOnOff(on int) string {
	if on == 1 {
		return "ON"
	}
	return "OFF"
}

func GetLightsSettings(lights []Light) {
	processLightOperation(lights, GetLightSettings, "Status")
}

func UpdateLightsSettings(lights []Light, settings LightDetail) {
	updateOperation := func(ip string) (*LightStatus, error) {
		return UpdateLightSettings(ip, settings)
	}
	processLightOperation(lights, updateOperation, "Update")
}

func FindLightByName(lights []LightConfig, name string) *LightConfig {
	for i := range lights {
		if lights[i].Name == name {
			return &lights[i]
		}
	}
	return nil
}

func GetAvailableLightNames(lights []LightConfig) string {
	var names []string
	for _, light := range lights {
		names = append(names, light.Name)
	}
	return strings.Join(names, ", ")
}

func ToLights(lightsConfig []LightConfig) []Light {
	var lights []Light
	for _, lightConfig := range lightsConfig {
		lights = append(lights, Light{
			Name: lightConfig.Name,
			IP:   lightConfig.IP,
		})
	}

	return lights
}
