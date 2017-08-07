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

var (
	specs = types.Specs{
		Appname: "root",
		Entrypoints: map[string]types.Entrypoint{
			"test": types.Entrypoint{
				Command:                 "sleep 9999",
				Ports:                   []types.Port{"6006/tcp"},
				HealthCheckPort:         6006,
				HealthCheckUrl:          "",
				HealthCheckExpectedCode: 200,
			},
		},
		Build: []string{""},
		Base:  image,
	}
	opts = &types.DeployOptions{
		Appname:    "root",
		Image:      image,
		Podname:    podname,
		Entrypoint: "test",
		Count:      3,
		Memory:     268435456,
		CPUQuota:   1,
	}
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

	// delete pod
	err := mockc.store.DeletePod(podname, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "still has nodes, delete the nodes first")

	// force delete
	err = mockc.store.DeletePod(podname, true)
	assert.NoError(t, err)

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
