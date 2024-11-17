package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

type Light struct {
	Name string `mapstructure:"name"`
	IP   string `mapstructure:"ip"`
}

type LightStatus struct {
	Lights         []LightDetail `json:"lights"`
	NumberOfLights int           `json:"numberOfLights"`
}

type LightDetail struct {
	On          int `json:"on"`
	Brightness  int `json:"brightness"`
	Temperature int `json:"temperature"`
}

func getStatus(lights []Light) {
	var wg sync.WaitGroup
	results := make(chan struct {
		err    error
		status *LightStatus
		name   string
	}, len(lights))

	done := make(chan struct{})
	go spinner(done)

	for _, light := range lights {
		wg.Add(1)
		go func(light Light) {
			defer wg.Done()
			status, err := fetchLightStatus(light)
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
			fmt.Printf("\rStatus of light %s: Error: %s\n", result.name, msg)
			continue
		}

		if len(result.status.Lights) > 0 {
			light := result.status.Lights[0]
			fmt.Printf("\rStatus of light %s:\n", result.name)
			fmt.Printf("  Power: %s\n", formatOnOff(light.On))
			fmt.Printf("  Brightness: %d%%\n", light.Brightness)
			fmt.Printf("  Temperature: %dK (mired: %d)\n",
				miredToKelvin(light.Temperature),
				light.Temperature)
		}
	}
}

func formatOnOff(on int) string {
	if on == 1 {
		return "ON"
	}
	return "OFF"
}

func miredToKelvin(mired int) int {
	// Mired is defined as 1 million divided by color temperature in Kelvin
	// So to get Kelvin from mired: K = 1000000/mired
	return 1000000 / mired
}

func fetchLightStatus(light Light) (*LightStatus, error) {
	statusURL := fmt.Sprintf("http://%s/elgato/lights", light.IP)
	resp, err := httpClient.Get(statusURL)
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

func isConnectionError(err error) bool {
	var netErr *net.OpError
	return errors.As(err, &netErr)
}
