package keylight

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Controller struct {
	client     *http.Client
	maxRetries int
	delay      time.Duration
}

func NewController() *Controller {
	return &Controller{
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		maxRetries: 3,
		delay:      100 * time.Millisecond,
	}
}

func (c *Controller) GetLight(ip string) (*LightStatus, error) {
	return retryHTTPCall(c.maxRetries, c.delay, func() (*LightStatus, error) {
		resp, err := c.client.Get(getLightsURL(ip))
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
	})
}

func (c *Controller) UpdateLight(ip string, settings LightDetail) (*LightStatus, error) {
	return retryHTTPCall(c.maxRetries, c.delay, func() (*LightStatus, error) {
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

		resp, err := c.client.Do(req)
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
	})
}

func retryHTTPCall(attempts int, initialDelay time.Duration, f func() (*LightStatus, error)) (*LightStatus, error) {
	var lastErr error
	delay := initialDelay

	for i := 0; i < attempts; i++ {
		result, err := f()
		if err == nil {
			return result, nil
		}
		lastErr = err
		time.Sleep(delay)
		delay *= 2 // exponential backoff
	}

	return nil, fmt.Errorf("after %d attempts, last error: %w", attempts, lastErr)
}

func getLightsURL(ip string) string {
	return fmt.Sprintf("http://%s/elgato/lights", ip)
}
