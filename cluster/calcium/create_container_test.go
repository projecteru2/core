package calcium

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.ricebook.net/platform/core/types"
)

func TestPullImage(t *testing.T) {
	initMockConfig()

	nodes, err := mockc.store.GetAllNodes()
	if err != nil || len(nodes) == 0 {
		t.Fatal(err)
	}

	if err := pullImage(nodes[0], image, 5*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestCreateContainerWithMemPrior(t *testing.T) {
	initMockConfig()

	// Create Container with memory prior
	testlogF("Create containers with memory prior")
	createCh, err := mockc.createContainerWithMemoryPrior(specs, opts)
	assert.NoError(t, err)
	ids := []string{}
	for msg := range createCh {
		assert.True(t, msg.Success)
		ids = append(ids, msg.ContainerID)
		fmt.Printf("Get Container ID: %s\n", msg.ContainerID)
	}
	assert.Equal(t, opts.Count, len(ids))

	// get containers
	clnt, _ := client.NewClient("http://127.0.0.1", "v1.29", mockDockerHTTPClient(), nil)
	cs := []types.Container{}
	for _, id := range ids {
		c := types.Container{
			ID:     id,
			Engine: clnt,
		}
		cs = append(cs, c)
		mockStore.On("GetContainer", id).Return(&c, nil)
	}
	mockStore.On("GetContainers", ids).Return(&cs, nil)

	// Remove Container
	testlogF("Remove containers")
	removeCh, err := mockc.RemoveContainer(ids)
	assert.NoError(t, err)
	for msg := range removeCh {
		fmt.Printf("ID: %s, Message: %s\n", msg.ContainerID, msg.Message)
		assert.True(t, msg.Success)
	}
}

func TestClean(t *testing.T) {
	initMockConfig()

	// delete pod, which will fail because there are remaining nodes
	err := mockc.store.RemovePod(podname)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "still has nodes")
}

func TestCreateContainerWithCPUPrior(t *testing.T) {
	initMockConfig()

	// update node
	mockStore.On("UpdateNode", mock.MatchedBy(func(input *types.Node) bool {
		return true
	})).Return(nil)

	// Create Container with memory prior
	testlogF("Create containers with memory prior")
	createCh, err := mockc.createContainerWithCPUPrior(specs, opts)
	assert.NoError(t, err)
	ids := []string{}
	for msg := range createCh {
		assert.True(t, msg.Success)
		ids = append(ids, msg.ContainerID)
		fmt.Printf("Get Container ID: %s\n", msg.ContainerID)
	}
}

func TestCreateContainerMemTimeoutError(t *testing.T) {
	initMockConfig()
	mockTimeoutError = true

	// update node
	mockStore.On("UpdateNode", mock.MatchedBy(func(input *types.Node) bool {
		return true
	})).Return(nil)

	// Create Container with memory prior
	testlogF("Create containers with memory prior")
	createCh, err := mockc.createContainerWithMemoryPrior(specs, opts)
	assert.NoError(t, err)
	for msg := range createCh {
		assert.False(t, msg.Success)
		assert.Contains(t, msg.Error, "context deadline exceeded")
	}

}

func TestCreateContainerCPUTimeoutError(t *testing.T) {
	initMockConfig()
	mockTimeoutError = true

	// update node
	mockStore.On("UpdateNode", mock.MatchedBy(func(input *types.Node) bool {
		return true
	})).Return(nil)

	// Create Container with cpu prior
	testlogF("Create containers with cpu prior")
	createCh, err := mockc.createContainerWithCPUPrior(specs, opts)
	assert.NoError(t, err)
	for msg := range createCh {
		assert.False(t, msg.Success)
		assert.Contains(t, msg.Error, "context deadline exceeded")
	}
}
