package models

import (
	"context"
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resources/volume/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestAlloc(t *testing.T) {
	ctx := context.Background()

	volume := newTestVolume(t)
	nodes := generateNodes(t, volume, 1)
	node := nodes[0]

	// invalid resource opt
	resourceOpts := &types.WorkloadResourceOpts{
		StorageRequest: -1,
	}
	_, _, err := volume.GetDeployArgs(ctx, node, 1, resourceOpts)
	assert.ErrorIs(t, err, types.ErrInvalidStorage)

	// invalid node
	resourceOpts = &types.WorkloadResourceOpts{
		VolumesLimit: generateVolumeBindings(t, []string{
			"/etc:/etc",
		}),
	}
	_, _, err = volume.GetDeployArgs(ctx, "fake-node", 1, resourceOpts)
	assert.ErrorIs(t, err, coretypes.ErrInvaildCount)

	// storage is not enough
	resourceOpts = &types.WorkloadResourceOpts{
		StorageRequest: 4 * units.TiB,
		VolumesLimit: generateVolumeBindings(t, []string{
			"/etc:/etc",
		}),
	}
	_, _, err = volume.GetDeployArgs(ctx, node, 2, resourceOpts)
	assert.ErrorIs(t, err, coretypes.ErrInsufficientResource)

	// normal case
	resourceOpts = &types.WorkloadResourceOpts{
		StorageRequest: 10 * units.GiB,
	}

	_, _, err = volume.GetDeployArgs(ctx, node, 100, resourceOpts)
	assert.Nil(t, err)

	// insufficient volume
	resourceOpts = &types.WorkloadResourceOpts{
		StorageRequest: 1 * units.GiB,
		VolumesLimit: generateVolumeBindings(t, []string{
			"AUTO:/dir0:rwm:1GiB",
			"/etc:etc",
		}),
	}
	_, _, err = volume.GetDeployArgs(ctx, node, 3, resourceOpts)
	assert.ErrorIs(t, err, coretypes.ErrInsufficientResource)

	// normal case
	resourceOpts = &types.WorkloadResourceOpts{
		StorageRequest: 1 * units.GiB,
		VolumesLimit: generateVolumeBindings(t, []string{
			"AUTO:/dir0:rwm:1GiB",
			"AUTO:/dir1:rw:1GiB",
		}),
	}
	_, _, err = volume.GetDeployArgs(ctx, node, 2, resourceOpts)
	assert.Nil(t, err)
}
