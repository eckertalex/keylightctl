package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/eckertalex/keylightctl/internal/api"
	"github.com/eckertalex/keylightctl/internal/utils"
)

type Light struct {
	Name string `mapstructure:"name"`
	IP   string `mapstructure:"ip"`
}

type LightProfile struct {
	Brightness  int `mapstructure:"brightness"`
	Temperature int `mapstructure:"temperature"`
}

type LightConfig struct {
	Light    `mapstructure:",squash"`
	Profiles map[string]LightProfile `mapstructure:"profiles"`
}

type lightOperation func(ip string) (*api.LightStatus, error)

func processLightOperation(lights []Light, operation lightOperation, operationName string) {
	var wg sync.WaitGroup
	results := make(chan struct {
		err    error
		status *api.LightStatus
		name   string
	}, len(lights))

	done := make(chan struct{})
	go utils.Spinner(done)

	for _, light := range lights {
		wg.Add(1)
		go func(light Light) {
			defer wg.Done()
			status, err := operation(light.IP)
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
			fmt.Printf("\r%s of light \"%s\": Error: %s\n", operationName, result.name, msg)
			continue
		}

		for _, light := range result.status.Lights {
			fmt.Printf("\rStatus of light \"%s\":\n", result.name)
			fmt.Printf("  Power: %s\n", formatOnOff(light.On))
			fmt.Printf("  Brightness: %d%%\n", light.Brightness)
			fmt.Printf("  Temperature: %dK (mired: %d)\n",
				utils.MiredToKelvin(light.Temperature),
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
	processLightOperation(lights, api.GetLightSettings, "Status")
}

func UpdateLightsSettings(lights []Light, settings api.LightDetail) {
	updateOperation := func(ip string) (*api.LightStatus, error) {
		return api.UpdateLightSettings(ip, settings)
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
