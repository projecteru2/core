package calcium

import (
	"testing"

	"gitlab.ricebook.net/platform/core/types"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

func TestRemoveContainer(t *testing.T) {
	initMockConfig()

	ids := []string{}
	clnt, _ := client.NewClient("http://127.0.0.1", "v1.29", mockDockerHTTPClient(), nil)
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
