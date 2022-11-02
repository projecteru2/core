package models

import (
	"context"
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resources/volume/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestAddNode(t *testing.T) {
	ctx := context.Background()

	volume := newTestVolume(t)
	nodes := generateNodes(t, volume, 1)

	resourceOpts := &types.NodeResourceOpts{
		Volumes: types.VolumeMap{"/data0": units.TiB},
		Storage: units.TiB,
	}

	// existent node
	_, err := volume.AddNode(ctx, nodes[0], resourceOpts)
	assert.ErrorIs(t, err, coretypes.ErrNodeExists)

	// normal case
	resourceInfo, err := volume.AddNode(ctx, "new-node", resourceOpts)
	assert.NoError(t, err)
	assert.Equal(t, resourceInfo.Capacity.Storage, resourceOpts.Storage)
	assert.Equal(t, resourceInfo.Capacity.Volumes, resourceOpts.Volumes)
	assert.Equal(t, resourceInfo.Usage, &types.NodeResourceArgs{Volumes: types.VolumeMap{"/data0": 0}, Disks: types.Disks(nil)})
}

func TestRemoveNode(t *testing.T) {
	ctx := context.Background()

	volume := newTestVolume(t)
	nodes := generateNodes(t, volume, 1)

	assert.Nil(t, volume.RemoveNode(ctx, nodes[0]))
	_, _, err := volume.GetNodeResourceInfo(ctx, nodes[0], nil, false)
	assert.ErrorIs(t, err, coretypes.ErrInvaildCount)

	assert.Nil(t, volume.RemoveNode(ctx, "invalid-node"))
}
