package calcium

import (
	"testing"

	"github.com/projecteru2/core/types"

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
			ID:      id,
			Podname: podname,
			Engine:  clnt,
		}
		mockStore.On("GetContainer", id).Return(&c, nil)
	}

	ch, err := mockc.RemoveContainer(ids, true)
	assert.NoError(t, err)

	for c := range ch {
		assert.True(t, c.Success)
	}
}
