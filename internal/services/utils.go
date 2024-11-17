package services

import (
	"errors"
	"net"
)

func formatOnOff(on int) string {
	if on == 1 {
		return "ON"
	}
	return "OFF"
}

func miredToKelvin(mired int) int {
	// Mired is defined as 1 million divided by color temperature in Kelvin
	// So to get Kelvin from mired: K = 1000000/mired
	return roundToNearest50(1000000 / mired)
}

func roundToNearest50(n int) int {
	return (n + 25) / 50 * 50
}

func isConnectionError(err error) bool {
	var netErr *net.OpError
	return errors.As(err, &netErr)
}
