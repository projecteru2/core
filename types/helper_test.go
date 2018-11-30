package types

import (
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
	assert.Equal(t, f(Round(a)), "2")
	a = 19.99998
	assert.Equal(t, f(Round(a)), "20")
}

func TestGetEndpointHost(t *testing.T) {
	endpoint := "xxxxx"
	s, err := getEndpointHost(endpoint)
	assert.Error(t, err)
	assert.Empty(t, s)

	endpoint = "tcp://ip"
	s, err = getEndpointHost(endpoint)
	assert.Error(t, err)
	assert.Empty(t, s)

	endpoint = "tcp://ip:port"
	s, err = getEndpointHost(endpoint)
	assert.NoError(t, err)
	assert.NotEmpty(t, s)
}
