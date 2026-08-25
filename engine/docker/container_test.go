package docker

import (
	"fmt"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	dockerapi "github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	dockermocks "github.com/projecteru2/core/engine/docker/mocks"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestVirtualizationRemoveReportsAMissingContainer(t *testing.T) {
	client := dockermocks.NewAPIClient(t)
	client.On("ContainerRemove", mock.Anything, "wid", mock.Anything).
		Return(dockerapi.ContainerRemoveResult{}, daemonNotFound("wid"))

	e := &Engine{client: client}
	assert.ErrorIs(t, e.VirtualizationRemove(t.Context(), "wid", true, true), coretypes.ErrWorkloadNotExists)
}

func TestVirtualizationInspectReportsAMissingContainer(t *testing.T) {
	client := dockermocks.NewAPIClient(t)
	client.On("ContainerInspect", mock.Anything, "wname", mock.Anything).
		Return(dockerapi.ContainerInspectResult{}, daemonNotFound("wname"))

	e := &Engine{client: client}
	_, err := e.VirtualizationInspect(t.Context(), "wname")
	assert.ErrorIs(t, err, coretypes.ErrWorkloadNotExists)
}

func TestVirtualizationUpdateResourceReportsAMissingContainer(t *testing.T) {
	client := dockermocks.NewAPIClient(t)
	client.On("ContainerUpdate", mock.Anything, "wid", mock.Anything).
		Return(dockerapi.ContainerUpdateResult{}, daemonNotFound("wid"))

	e := &Engine{client: client}
	assert.ErrorIs(t, e.VirtualizationUpdateResource(t.Context(), "wid", boundEngineParams()), coretypes.ErrWorkloadNotExists)
}

func TestVirtualizationUpdateResourceReportsAContainerBeingRemoved(t *testing.T) {
	client := dockermocks.NewAPIClient(t)
	client.On("ContainerUpdate", mock.Anything, "wid", mock.Anything).
		Return(dockerapi.ContainerUpdateResult{}, daemonConflict("wid"))

	e := &Engine{client: client}
	assert.ErrorIs(t, e.VirtualizationUpdateResource(t.Context(), "wid", boundEngineParams()), coretypes.ErrWorkloadRemoving)
}

func TestRawArgs(t *testing.T) {
	assert := assert.New(t)

	r1, err := loadRawArgs([]byte(``))
	assert.NoError(err)
	assert.NotEqual(r1.StorageOpt, nil)
	assert.Equal(len(r1.StorageOpt), 0)
	assert.NotEqual(r1.CapAdd, nil)
	assert.Equal(len(r1.CapAdd), 0)
	assert.NotEqual(r1.CapDrop, nil)
	assert.Equal(len(r1.CapDrop), 0)
	assert.NotEqual(r1.Ulimits, nil)
	assert.Equal(len(r1.Ulimits), 0)

	r2, err := loadRawArgs([]byte(`{"storage_opt": null, "cap_add": null, "cap_drop": null, "ulimits": null}`))
	assert.NoError(err)
	assert.NotEqual(r2.StorageOpt, nil)
	assert.Equal(len(r2.StorageOpt), 0)
	assert.NotEqual(r2.CapAdd, nil)
	assert.Equal(len(r2.CapAdd), 0)
	assert.NotEqual(r2.CapDrop, nil)
	assert.Equal(len(r2.CapDrop), 0)
	assert.NotEqual(r2.Ulimits, nil)
	assert.Equal(len(r2.Ulimits), 0)

	_, err = loadRawArgs([]byte(`{"storage_opt": null, "cap_add": null, "cap_drop": null, "ulimits"}`))
	assert.Error(err)
}

func daemonNotFound(name string) error {
	return fmt.Errorf("Error response from daemon: No such container: %s: %w", name, cerrdefs.ErrNotFound)
}

func daemonConflict(name string) error {
	return fmt.Errorf("Error response from daemon: container %s is marked for removal and cannot be update: %w", name, cerrdefs.ErrConflict)
}

func boundEngineParams() resourcetypes.Resources {
	return resourcetypes.Resources{"cpumem": {"cpu": 1.0, "cpu_map": map[string]int64{"0": 100}}}
}
