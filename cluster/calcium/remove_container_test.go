package calcium

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveContainer(t *testing.T) {
	initMockConfig()

	ids := []string{}
	for i := 0; i < 5; i++ {
		ids = append(ids, mockContainerID())
	}
	ch, err := mockc.RemoveContainer(ids)
	if err != nil {
		t.Error(err)
		return
	}
	for c := range ch {
		assert.True(t, c.Success)
	}
}
