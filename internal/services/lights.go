package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/eckertalex/keylightctl/internal/api"
)

type Light struct {
	Name string
	IP   string
}

func GetLightsSettings(lights []Light) {
	var wg sync.WaitGroup
	results := make(chan struct {
		err    error
		status *api.LightStatus
		name   string
	}, len(lights))

	done := make(chan struct{})
	go spinner(done)

	for _, light := range lights {
		wg.Add(1)
		go func(light Light) {
			defer wg.Done()
			status, err := api.GetLightSettings(light.IP)
			results <- struct {
				err    error
				status *api.LightStatus
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
			fmt.Printf("\rStatus of light \"%s\": Error: %s\n", result.name, msg)
			continue
		}

		for _, light := range result.status.Lights {
			fmt.Printf("\rStatus of light \"%s\":\n", result.name)
			fmt.Printf("  Power: %s\n", formatOnOff(light.On))
			fmt.Printf("  Brightness: %d%%\n", light.Brightness)
			fmt.Printf("  Temperature: %dK (mired: %d)\n",
				miredToKelvin(light.Temperature),
				light.Temperature)
		}
	}
}

func UpdateLightsSettings(lights []Light, settings api.LightDetail) {
	var wg sync.WaitGroup
	results := make(chan struct {
		err    error
		status *api.LightStatus
		name   string
	}, len(lights))

	done := make(chan struct{})
	go spinner(done)

	for _, light := range lights {
		wg.Add(1)
		go func(light Light) {
			defer wg.Done()
			status, err := api.UpdateLightSettings(light.IP, settings)
			results <- struct {
				err    error
				status *api.LightStatus
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
			fmt.Printf("\rUpdate of light %s: Error: %s\n", result.name, msg)
			continue
		}

		for _, light := range result.status.Lights {
			fmt.Printf("\rStatus of light \"%s\":\n", result.name)
			fmt.Printf("  Power: %s\n", formatOnOff(light.On))
			fmt.Printf("  Brightness: %d%%\n", light.Brightness)
			fmt.Printf("  Temperature: %dK (mired: %d)\n",
				miredToKelvin(light.Temperature),
				light.Temperature)
		}
	}
}
