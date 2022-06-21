package types

import "math"

// Round for float64 to int
func Round(f float64) float64 {
	return math.Round(f*1000000000) / 1000000000
}

// RoundToInt for float64 to int
func RoundToInt(f float64) int64 {
	return int64(math.Round(f))
}
