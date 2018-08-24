package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveImage(t *testing.T) {
	initMockConfig()

	images := []string{image}
	ch, err := mockc.RemoveImage(context.Background(), podname, nodename, images, false)
	assert.NoError(t, err)

	for c := range ch {
		assert.True(t, c.Success)
	}
}
