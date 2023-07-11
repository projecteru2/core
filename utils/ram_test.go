package utils

import (
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
)

func TestParseRAMInHuman(t *testing.T) {
	size, err := ParseRAMInHuman("")
	assert.Nil(t, err)
	assert.EqualValues(t, 0, size)

	size, err = ParseRAMInHuman("1")
	assert.Nil(t, err)
	assert.EqualValues(t, 1, size)

	size, err = ParseRAMInHuman("-1")
	assert.Nil(t, err)
	assert.EqualValues(t, -1, size)

	size, err = ParseRAMInHuman("hhhh")
	assert.NotNil(t, err)

	size, err = ParseRAMInHuman("1G")
	assert.Nil(t, err)
	assert.EqualValues(t, units.GiB, size)

	size, err = ParseRAMInHuman("-1T")
	assert.Nil(t, err)
	assert.EqualValues(t, -units.TiB, size)
}
