package calcium

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveImage(t *testing.T) {
	initMockConfig()

	images := []string{image}
	ch, err := mockc.RemoveImage(podname, nodename, images)
	assert.NoError(t, err)

	for c := range ch {
		assert.True(t, c.Success)
	}
}
