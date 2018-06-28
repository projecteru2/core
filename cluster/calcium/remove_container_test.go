package calcium

import (
	"context"
	"testing"

	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func TestRemoveContainer(t *testing.T) {
	initMockConfig()

	IDs := []string{}
	clnt := mockDockerClient()
	for i := 0; i < 5; i++ {
		IDs = append(IDs, mockContainerID())
	}
	for _, ID := range IDs {
		c := types.Container{
			ID:      ID,
			Podname: podname,
			Engine:  clnt,
		}
		mockStore.On("GetContainer", ID).Return(&c, nil)
	}

	ch, err := mockc.RemoveContainer(context.Background(), IDs, true)
	assert.NoError(t, err)

	for c := range ch {
		assert.True(t, c.Success)
	}
}
