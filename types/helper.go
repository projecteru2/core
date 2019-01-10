package types

import (
	"math"
)

// Round for float64 to int
func Round(f float64) float64 {
	return math.Round(f*100) / 100
}
