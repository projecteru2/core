package models

import (
	"context"
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resources/volume/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestGetNodesDeployCapacity(t *testing.T) {
	ctx := context.Background()

	volume := newTestVolume(t)
	nodes := generateNodes(t, volume, 10)

	// invalid opts
	resourceOpts := &types.WorkloadResourceOpts{
		StorageRequest: -1,
	}

	_, _, err := volume.GetNodesDeployCapacity(ctx, nodes, resourceOpts)
	assert.ErrorIs(t, err, types.ErrInvalidStorage)

	// invalid node
	resourceOpts = &types.WorkloadResourceOpts{
		StorageRequest: 1,
	}
	_, _, err = volume.GetNodesDeployCapacity(ctx, []string{"invalid"}, resourceOpts)
	assert.ErrorIs(t, err, coretypes.ErrBadCount)

	// no volume request
	resourceOpts = &types.WorkloadResourceOpts{
		StorageLimit: 100 * units.GiB,
	}
	_, total, err := volume.GetNodesDeployCapacity(ctx, nodes, resourceOpts)
	assert.NoError(t, err)
	assert.Equal(t, total, 100)

	// no storage request
	resourceOpts = &types.WorkloadResourceOpts{
		VolumesLimit: generateVolumeBindings(t, []string{
			"AUTO:/dir0:rwm:1G",
		}),
	}
	_, total, err = volume.GetNodesDeployCapacity(ctx, nodes, resourceOpts)
	assert.NoError(t, err)
	assert.Equal(t, total, 20)

	// mixed
	resourceOpts = &types.WorkloadResourceOpts{
		VolumesLimit: generateVolumeBindings(t, []string{
			"AUTO:/dir0:rwm:1G",
			"AUTO:/dir1:rwm:1G",
		}),
		StorageRequest: 1 * units.TiB,
	}
	_, total, err = volume.GetNodesDeployCapacity(ctx, nodes, resourceOpts)
	assert.NoError(t, err)
	assert.Equal(t, total, 10)
}
