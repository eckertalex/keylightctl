package utils

import (
	"fmt"
)

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
