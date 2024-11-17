package api

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

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
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
