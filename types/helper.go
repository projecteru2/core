package types

import (
	"bytes"
	"math"
)

// Round for float64 to int
func Round(f float64) float64 {
	return math.Round(f*1000000) / 1000000
}

// HookOutput output hooks output
func HookOutput(outputs []*bytes.Buffer) []byte {
	r := []byte{}
	for _, m := range outputs {
		r = append(r, m.Bytes()...)
	}
	return r
}
