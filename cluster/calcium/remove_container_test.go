package calcium

import (
	"testing"

	"gitlab.ricebook.net/platform/core/types"

	"github.com/stretchr/testify/assert"
)

func TestRemoveContainer(t *testing.T) {
	initMockConfig()

	ids := []string{}
	clnt := mockDockerClient()
	for i := 0; i < 5; i++ {
		ids = append(ids, mockContainerID())
	}
	for _, id := range ids {
		c := types.Container{
			ID:     id,
			Engine: clnt,
		}
		mockStore.On("GetContainer", id).Return(&c, nil)
	}

	ch, err := mockc.RemoveContainer(ids)
	assert.NoError(t, err)

	for c := range ch {
		assert.True(t, c.Success)
	}
}
