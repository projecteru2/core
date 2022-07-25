package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetOutboundAddress(t *testing.T) {
	bind := "1.1.1.1:1234"
	addr, err := GetOutboundAddress(bind, "8.8.8.8:80")
	assert.NoError(t, err)
	assert.Contains(t, addr, "1234")
}
