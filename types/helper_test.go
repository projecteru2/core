package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
