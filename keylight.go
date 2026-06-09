package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type LightConfig struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type controller struct {
	client     *http.Client
	maxRetries int
	delay      time.Duration
}

func newController() *controller {
	return &controller{
		client:     &http.Client{Timeout: 3 * time.Second},
		maxRetries: 3,
		delay:      100 * time.Millisecond,
	}
}

func (c *controller) getLight(ip string) (*LightStatus, error) {
	return retryHTTP(c.maxRetries, c.delay, func() (*LightStatus, error) {
		resp, err := c.client.Get(lightsURL(ip))
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		var s LightStatus
		if err := json.Unmarshal(body, &s); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}

		return &s, nil
	})
}

func (c *controller) updateLight(ip string, settings LightDetail) (*LightStatus, error) {
	return retryHTTP(c.maxRetries, c.delay, func() (*LightStatus, error) {
		body, err := json.Marshal(LightStatus{Lights: []LightDetail{settings}})
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}

		req, err := http.NewRequest(http.MethodPut, lightsURL(ip), bytes.NewBuffer(body))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
		}

		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		var s LightStatus
		if err := json.Unmarshal(respBody, &s); err != nil {
			return nil, fmt.Errorf("parsing response: %w", err)
		}

		return &s, nil
	})
}

func retryHTTP(attempts int, delay time.Duration, f func() (*LightStatus, error)) (*LightStatus, error) {
	var lastErr error
	for range attempts {
		if result, err := f(); err == nil {
			return result, nil
		} else {
			lastErr = err
		}

		time.Sleep(delay)
		delay *= 2
	}

	return nil, fmt.Errorf("after %d attempts: %w", attempts, lastErr)
}

func lightsURL(ip string) string {
	return fmt.Sprintf("http://%s/elgato/lights", ip)
}

func miredToKelvin(mired int) int {
	return roundToNearest50(1_000_000 / mired)
}

func kelvinToMired(kelvin int) int {
	return 1_000_000 / kelvin
}

func roundToNearest50(n int) int {
	return (n + 25) / 50 * 50
}
