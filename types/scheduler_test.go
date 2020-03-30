package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetResource(t *testing.T) {
	assert.Equal(t, GetResourceType(true, true), ResourceCPU|ResourceVolume)
	assert.Equal(t, GetResourceType(true, false), ResourceCPU)
	assert.Equal(t, GetResourceType(false, false), ResourceMemory)
}
