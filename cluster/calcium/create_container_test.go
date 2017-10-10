package calcium

import (
	"fmt"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPullImage(t *testing.T) {
	initMockConfig()

	nodes, err := mockc.store.GetAllNodes()
	if err != nil || len(nodes) == 0 {
		t.Fatal(err)
	}

	if err := pullImage(nodes[0], image); err != nil {
		t.Fatal(err)
	}
}

func TestCreateContainerWithMemPrior(t *testing.T) {
	initMockConfig()

	// Create Container with memory prior
	testlogF("Create containers with memory prior")
	createCh, err := mockc.createContainerWithMemoryPrior(opts)
	assert.NoError(t, err)
	ids := []string{}
	for msg := range createCh {
		assert.True(t, msg.Success)
		ids = append(ids, msg.ContainerID)
		fmt.Printf("Get Container ID: %s\n", msg.ContainerID)
	}
	assert.Equal(t, opts.Count, len(ids))

	// get containers
	clnt := mockDockerClient()
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
	removeCh, err := mockc.RemoveContainer(ids, true)
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
	createCh, err := mockc.createContainerWithCPUPrior(opts)
	assert.NoError(t, err)
	ids := []string{}
	for msg := range createCh {
		assert.True(t, msg.Success)
		ids = append(ids, msg.ContainerID)
		fmt.Printf("Get Container ID: %s\n", msg.ContainerID)
	}
}
