package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- conversion ---

func TestRoundToNearest50(t *testing.T) {
	tests := []struct{ in, want int }{
		{0, 0},
		{24, 0},
		{25, 50},
		{50, 50},
		{74, 50},
		{75, 100},
		{3003, 3000},
		{3025, 3050},
	}
	for _, tt := range tests {
		if got := roundToNearest50(tt.in); got != tt.want {
			t.Errorf("roundToNearest50(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestMiredToKelvin(t *testing.T) {
	tests := []struct {
		mired, kelvin int
	}{
		{200, 5000},
		{250, 4000},
		{333, 3000},
		{344, 2900},
	}
	for _, tt := range tests {
		if got := miredToKelvin(tt.mired); got != tt.kelvin {
			t.Errorf("miredToKelvin(%d) = %d, want %d", tt.mired, got, tt.kelvin)
		}
	}
}

func TestKelvinToMired(t *testing.T) {
	tests := []struct {
		kelvin, mired int
	}{
		{5000, 200},
		{4000, 250},
		{3000, 333},
		{2900, 344},
	}
	for _, tt := range tests {
		if got := kelvinToMired(tt.kelvin); got != tt.mired {
			t.Errorf("kelvinToMired(%d) = %d, want %d", tt.kelvin, got, tt.mired)
		}
	}
}

func TestKelvinMiredRoundTrip(t *testing.T) {
	// High Kelvin values (e.g. 6500K) lose precision due to integer truncation
	// on the mired scale; only values with a near-round mired equivalent survive.
	for _, k := range []int{2900, 3000, 3500, 4000, 4500, 5000, 5500, 6000} {
		if got := miredToKelvin(kelvinToMired(k)); got != k {
			t.Errorf("round-trip(%dK): got %dK", k, got)
		}
	}
}

// --- HTTP client ---

func lightServer(t *testing.T, status LightStatus) (ip string, requests *[]*http.Request) {
	t.Helper()
	var reqs []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs = append(reqs, r)
		json.NewEncoder(w).Encode(status)
	}))
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://"), &reqs
}

func TestControllerGetLight(t *testing.T) {
	want := LightStatus{Lights: []LightDetail{{On: 1, Brightness: 50, Temperature: 250}}}
	ip, reqs := lightServer(t, want)

	c := newController()
	got, err := c.getLight(ip)
	if err != nil {
		t.Fatalf("getLight: %v", err)
	}
	if len(*reqs) != 1 || (*reqs)[0].Method != http.MethodGet {
		t.Errorf("expected 1 GET, got %d requests", len(*reqs))
	}
	if got.Lights[0] != want.Lights[0] {
		t.Errorf("got %+v, want %+v", got.Lights[0], want.Lights[0])
	}
}

func TestControllerUpdateLight(t *testing.T) {
	response := LightStatus{Lights: []LightDetail{{On: 1, Brightness: 75, Temperature: 250}}}
	ip, reqs := lightServer(t, response)

	c := newController()
	settings := LightDetail{On: 1, Brightness: 75, Temperature: 250}
	got, err := c.updateLight(ip, settings)
	if err != nil {
		t.Fatalf("updateLight: %v", err)
	}
	if len(*reqs) != 1 || (*reqs)[0].Method != http.MethodPut {
		t.Errorf("expected 1 PUT, got %d requests", len(*reqs))
	}
	if (*reqs)[0].Header.Get("Content-Type") != "application/json" {
		t.Errorf("missing Content-Type: application/json")
	}
	if got.Lights[0] != response.Lights[0] {
		t.Errorf("got %+v, want %+v", got.Lights[0], response.Lights[0])
	}
}

func TestControllerUpdateLightNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	c := newController()
	c.maxRetries = 1
	c.delay = time.Millisecond
	_, err := c.updateLight(strings.TrimPrefix(srv.URL, "http://"), LightDetail{On: 1})
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
}

// --- retry ---

func TestRetryHTTPSucceedsOnSecondAttempt(t *testing.T) {
	calls := 0
	result, err := retryHTTP(3, time.Millisecond, func() (*LightStatus, error) {
		calls++
		if calls < 2 {
			return nil, errors.New("transient")
		}
		return &LightStatus{Lights: []LightDetail{{On: 1}}}, nil
	})
	if err != nil {
		t.Fatalf("retryHTTP: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
	if result.Lights[0].On != 1 {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestRetryHTTPExhausted(t *testing.T) {
	calls := 0
	_, err := retryHTTP(3, time.Millisecond, func() (*LightStatus, error) {
		calls++
		return nil, errors.New("always fails")
	})
	if err == nil {
		t.Fatal("expected error after exhausted retries, got nil")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}
