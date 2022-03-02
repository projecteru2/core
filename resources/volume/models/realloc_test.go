package models

import (
	"context"
	"testing"

	"github.com/docker/go-units"
	"github.com/sanity-io/litter"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resources/volume/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestRealloc(t *testing.T) {
	ctx := context.Background()

	volume := newTestVolume(t)
	nodes := generateNodes(t, volume, 1)
	node := nodes[0]

	plan := types.VolumePlan{}
	err := plan.UnmarshalJSON([]byte(`
{
	"AUTO:/dir0:rw:100GiB": {
        "/data0": 107374182400
      },
      "AUTO:/dir1:mrw:100GiB": {
        "/data2": 1099511627776
      },
      "AUTO:/dir2:rw:0": {
        "/data0": 0
      }
}
`))
	assert.Nil(t, err)
	bindings := generateVolumeBindings(t, []string{
		"AUTO:/dir0:rw:100GiB",
		"AUTO:/dir1:mrw:100GiB",
		"AUTO:/dir2:rw:0",
	})

	originResourceArgs := &types.WorkloadResourceArgs{
		VolumesRequest:    bindings,
		VolumesLimit:      bindings,
		VolumePlanRequest: plan,
		VolumePlanLimit:   plan,
		StorageRequest:    0,
		StorageLimit:      0,
	}

	_, _, err = volume.SetNodeResourceUsage(ctx, node, nil, nil, []*types.WorkloadResourceArgs{originResourceArgs}, true, true)
	assert.Nil(t, err)

	// non-existent node
	_, _, _, err = volume.GetReallocArgs(ctx, "invalid-node", originResourceArgs, &types.WorkloadResourceOpts{})
	assert.ErrorIs(t, err, coretypes.ErrBadCount)

	// invalid resource opts
	opts := &types.WorkloadResourceOpts{
		VolumesRequest: generateVolumeBindings(t, []string{
			"AUTO:/dir0:rw:100GiB",
			"AUTO:/dir1:mrw:100GiB",
			"AUTO:/dir2:rw:0",
		}),
		VolumesLimit:   nil,
		StorageRequest: -1,
		StorageLimit:   -1,
	}
	_, _, _, err = volume.GetReallocArgs(ctx, node, originResourceArgs, opts)
	assert.ErrorIs(t, err, types.ErrInvalidStorage)

	// insufficient storage
	bindings = generateVolumeBindings(t, []string{
		"AUTO:/dir1:mrw:100GiB",
	})
	opts = &types.WorkloadResourceOpts{
		VolumesRequest: bindings,
		VolumesLimit:   bindings,
		StorageRequest: 2 * units.TiB,
		StorageLimit:   2 * units.TiB,
	}
	_, _, _, err = volume.GetReallocArgs(ctx, node, originResourceArgs, opts)
	assert.ErrorIs(t, err, types.ErrInsufficientResource)

	// insufficient volume
	bindings = generateVolumeBindings(t, []string{
		"AUTO:/dir1:mrw:1TiB",
	})
	opts = &types.WorkloadResourceOpts{
		VolumesRequest: bindings,
		VolumesLimit:   bindings,
		StorageRequest: 0,
		StorageLimit:   0,
	}
	_, _, _, err = volume.GetReallocArgs(ctx, node, originResourceArgs, opts)
	assert.ErrorIs(t, err, types.ErrInsufficientResource)

	// normal case
	bindings = generateVolumeBindings(t, []string{
		"AUTO:/dir1:mrw:100GiB",
	})
	opts = &types.WorkloadResourceOpts{
		VolumesRequest: bindings,
		VolumesLimit:   bindings,
		StorageRequest: units.GiB,
		StorageLimit:   units.GiB,
	}

	_, _, finalWorkloadResourceArgs, err := volume.GetReallocArgs(ctx, node, originResourceArgs, opts)
	assert.Nil(t, err)
	plan = types.VolumePlan{}
	assert.Nil(t, plan.UnmarshalJSON([]byte(`
{
	"AUTO:/dir0:rw:100GiB": {
        "/data0": 107374182400
      },
      "AUTO:/dir1:mrw:200GiB": {
        "/data2": 1099511627776
      },
      "AUTO:/dir2:rw:0": {
        "/data0": 0
      }
}
`)))
	assert.Equal(t, litter.Sdump(plan), litter.Sdump(finalWorkloadResourceArgs.VolumePlanRequest))
}
