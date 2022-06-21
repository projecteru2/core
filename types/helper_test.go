package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRound(t *testing.T) {
	a := 0.0199999998
	assert.InDelta(t, Round(a), 0.02, 1e-5)
	a = 0.1999998
	assert.InDelta(t, Round(a), 0.2, 1e-5)
	a = 1.999998
	assert.InDelta(t, Round(a), 1.999998, 1e-6)
	a = 19.99998
	assert.InDelta(t, (Round(a)), 19.99998, 1e-6)
}

func TestRoundToInt(t *testing.T) {
	a := 0.0199999998
	assert.EqualValues(t, RoundToInt(a), 0)
	a = 0.1999998
	assert.EqualValues(t, RoundToInt(a), 0)
	a = 1.999998
	assert.EqualValues(t, RoundToInt(a), 2)
}
