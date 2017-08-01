package calcium

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveImage(t *testing.T) {
	initMockConfig()

	images := []string{image}
	ch, err := mockc.RemoveImage(podname, nodename, images)
	if err != nil {
		t.Error(err)
		return
	}
	for c := range ch {
		assert.True(t, c.Success)
	}
}
