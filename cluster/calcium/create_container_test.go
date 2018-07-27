package calcium

import (
	"context"
	"fmt"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPullImage(t *testing.T) {
	initMockConfig()

	ctx := context.Background()
	nodes, err := mockc.store.GetAllNodes(ctx)
	if err != nil || len(nodes) == 0 {
		t.Fatal(err)
	}

	if err := pullImage(ctx, nodes[0], image, ""); err != nil {
		t.Fatal(err)
	}
}

func TestCreateContainerWithMemPrior(t *testing.T) {
	initMockConfig()
	ctx := context.Background()
	// Create Container with memory prior
	testlogF("Create containers with memory prior")
	createCh, err := mockc.createContainerWithMemoryPrior(ctx, opts)
	assert.NoError(t, err)
	IDs := []string{}
	for msg := range createCh {
		assert.True(t, msg.Success)
		IDs = append(IDs, msg.ContainerID)
		fmt.Printf("Get Container ID: %s\n", msg.ContainerID)
	}
	assert.Equal(t, opts.Count, len(IDs))

	// get containers
	clnt := mockDockerClient()
	cs := []types.Container{}
	for _, ID := range IDs {
		c := types.Container{
			ID:      ID,
			Podname: podname,
			Engine:  clnt,
		}
		cs = append(cs, c)
		mockStore.On("GetContainer", ID).Return(&c, nil)
	}
	mockStore.On("GetContainers", IDs).Return(&cs, nil)

	// Remove Container
	testlogF("Remove containers")
	removeCh, err := mockc.RemoveContainer(ctx, IDs, true)
	assert.NoError(t, err)
	for msg := range removeCh {
		fmt.Printf("ID: %s, Message: %s\n", msg.ContainerID, msg.Message)
		assert.True(t, msg.Success)
	}
}

func TestClean(t *testing.T) {
	initMockConfig()

	// delete pod, which will fail because there are remaining nodes
	err := mockc.store.RemovePod(context.Background(), podname)
	assert.EqualError(t, types.IsDetailedErr(err), types.ErrPodHasNodes.Error())
}

func TestCreateContainerWithCPUPrior(t *testing.T) {
	initMockConfig()

	// update node
	mockStore.On("UpdateNode", mock.MatchedBy(func(input *types.Node) bool {
		return true
	})).Return(nil)

	// Create Container with memory prior
	testlogF("Create containers with memory prior")
	createCh, err := mockc.createContainerWithCPUPrior(context.Background(), opts)
	assert.NoError(t, err)
	IDs := []string{}
	for msg := range createCh {
		assert.True(t, msg.Success)
		IDs = append(IDs, msg.ContainerID)
		fmt.Printf("Get Container ID: %s\n", msg.ContainerID)
	}
}
