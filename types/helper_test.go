package types

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRound(t *testing.T) {
	f := func(f float64) string {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	a := 0.0199999998
	assert.Equal(t, f(Round(a)), "0.02")
	a = 0.1999998
	assert.Equal(t, f(Round(a)), "0.2")
	a = 1.999998
	assert.Equal(t, f(Round(a)), "1.999998")
	a = 19.99998
	assert.Equal(t, f(Round(a)), "19.99998")
}

func TestHookOutput(t *testing.T) {
	test := []*bytes.Buffer{bytes.NewBufferString("a"), bytes.NewBufferString("b")}
	r := HookOutput(test)
	assert.NotEmpty(t, r)
	assert.Equal(t, string(r), "ab")
}
