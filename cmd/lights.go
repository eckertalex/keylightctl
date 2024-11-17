package cmd

import (
	"context"
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

func getStatus(lights []Light) {
	var wg sync.WaitGroup
	results := make(chan struct {
		name   string
		status string
	}, len(lights))
	done := make(chan struct{})

	go spinner(done)

	for _, light := range lights {
		wg.Add(1)
		go func(light Light) {
			defer wg.Done()

			status, err := fetchLightStatus(light)
			if err != nil {
				msg := "unknown error"
				switch {
				case errors.Is(err, context.DeadlineExceeded) ||
					errors.Is(err, context.Canceled):
					msg = "timeout while connecting"
				case errors.Is(err, io.EOF):
					msg = "connection closed unexpectedly"
				case isConnectionError(err):
					msg = "failed to connect"
				}

				results <- struct {
					name   string
					status string
				}{light.Name, fmt.Sprintf("Error: %s", msg)}
				return
			}

			results <- struct {
				name   string
				status string
			}{light.Name, status}
		}(light)
	}

	go func() {
		wg.Wait()
		close(results)
		close(done)
	}()

	for result := range results {
		fmt.Printf("\rStatus of light %s: %s\n", result.name, result.status)
	}
}

func fetchLightStatus(light Light) (string, error) {
	statusURL := fmt.Sprintf("http://%s/elgato/lights", light.IP)
	resp, err := httpClient.Get(statusURL)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response failed: %w", err)
	}

	return string(body), nil
}

func isConnectionError(err error) bool {
	var netErr *net.OpError
	return errors.As(err, &netErr)
}
