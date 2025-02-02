package keylight

func MiredToKelvin(mired int) int {
	// Mired is defined as 1 million divided by color temperature in Kelvin
	// So to get Kelvin from mired: K = 1000000/mired
	return roundToNearest50(1000000 / mired)
}

func KelvinToMired(kelvin int) int {
	// Mired is defined as 1,000,000 / Kelvin
	return roundToNearest50(1000000 / kelvin)
}

func roundToNearest50(n int) int {
	return (n + 25) / 50 * 50
}
