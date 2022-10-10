package models

import (
	"context"
	"testing"

	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resources/volume/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestGetNodeResourceInfo(t *testing.T) {
	ctx := context.Background()

	volume := newTestVolume(t)
	nodes := generateNodes(t, volume, 10)
	node := nodes[0]

	// invalid node
	_, _, err := volume.GetNodeResourceInfo(ctx, "xxx", nil, false)
	assert.True(t, errors.Is(err, coretypes.ErrBadCount))

	// normal case
	resourceInfo, diffs, err := volume.GetNodeResourceInfo(ctx, node, nil, false)
	assert.Nil(t, err)
	assert.Equal(t, len(diffs), 3)

	bindings := generateVolumeBindings(t, []string{
		"AUTO:/dir0:rw:1GiB",
	})

	resourceInfo, diffs, err = volume.GetNodeResourceInfo(ctx, node, &types.WorkloadResourceArgsMap{
		"x-workload": {
			VolumesRequest: bindings,
			VolumesLimit:   bindings,
			VolumePlanRequest: types.VolumePlan{
				bindings[0]: types.VolumeMap{"/data0": units.GiB},
			},
			VolumePlanLimit: types.VolumePlan{
				bindings[0]: types.VolumeMap{"/data0": units.GiB},
			},
			StorageRequest: units.GiB,
			StorageLimit:   units.GiB,
		},
	}, true)
	assert.Nil(t, err)
	assert.Equal(t, len(diffs), 3)
	assert.Equal(t, resourceInfo.Usage, &types.NodeResourceArgs{
		Volumes: types.VolumeMap{
			"/data0": units.GiB,
		},
		Storage: units.GiB,
		Disks:   types.Disks{},
	})
}

func TestSetNodeResourceInfo(t *testing.T) {
	ctx := context.Background()

	volume := newTestVolume(t)
	nodes := generateNodes(t, volume, 10)
	node := nodes[0]

	resourceInfo, _, err := volume.GetNodeResourceInfo(ctx, node, nil, false)
	assert.Nil(t, err)

	err = volume.SetNodeResourceInfo(ctx, "node-x", resourceInfo.Capacity, resourceInfo.Usage)
	assert.Nil(t, err)
}

func TestSetNodeResourceUsage(t *testing.T) {
	ctx := context.Background()

	volume := newTestVolume(t)
	nodes := generateNodes(t, volume, 10)
	node := nodes[0]

	_, _, err := volume.GetNodeResourceInfo(ctx, node, nil, false)
	assert.Nil(t, err)

	var after *types.NodeResourceArgs

	nodeResourceOpts := &types.NodeResourceOpts{
		Volumes: types.VolumeMap{"/data0": units.GiB},
		Storage: units.GiB,
		Disks:   types.Disks{},
	}

	nodeResourceArgs := &types.NodeResourceArgs{
		Volumes: types.VolumeMap{"/data0": units.GiB},
		Storage: units.GiB,
		Disks:   types.Disks{},
	}

	workloadResourceArgs := []*types.WorkloadResourceArgs{
		{
			VolumePlanRequest: types.VolumePlan{
				generateVolumeBindings(t, []string{"AUTO:/dir0:rw:1GiB"})[0]: types.VolumeMap{"/data0": units.GiB},
			},
			StorageRequest: units.GiB,
		},
	}

	originResourceUsage := &types.NodeResourceArgs{
		Volumes: types.VolumeMap{"/data0": 200 * units.GiB, "/data1": 300 * units.GiB},
		Storage: 500 * units.GiB,
		Disks:   types.Disks{},
	}

	afterSetNodeResourceUsageDelta := &types.NodeResourceArgs{
		Volumes: types.VolumeMap{
			"/data0": 201 * units.GiB,
			"/data1": 300 * units.GiB,
		},
		Storage: 501 * units.GiB,
		Disks:   types.Disks{},
	}

	_, after, err = volume.SetNodeResourceUsage(ctx, node, nodeResourceOpts, nil, nil, true, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDelta)

	_, after, err = volume.SetNodeResourceUsage(ctx, node, nodeResourceOpts, nil, nil, true, false)
	assert.Nil(t, err)
	assert.Equal(t, originResourceUsage, after)

	_, after, err = volume.SetNodeResourceUsage(ctx, node, nil, nodeResourceArgs, nil, true, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDelta)

	_, after, err = volume.SetNodeResourceUsage(ctx, node, nil, nodeResourceArgs, nil, true, false)
	assert.Nil(t, err)
	assert.Equal(t, originResourceUsage, after)

	_, after, err = volume.SetNodeResourceUsage(ctx, node, nil, nil, workloadResourceArgs, true, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDelta)

	_, after, err = volume.SetNodeResourceUsage(ctx, node, nil, nil, workloadResourceArgs, true, false)
	assert.Nil(t, err)
	assert.Equal(t, originResourceUsage, after)

	_, after, err = volume.SetNodeResourceUsage(ctx, node, nil, nil, nil, true, false)
	assert.Nil(t, err)
	assert.Equal(t, originResourceUsage, after)

	_, after, err = volume.SetNodeResourceUsage(ctx, node, nil, nodeResourceArgs, nil, false, true)
	assert.Nil(t, err)
	assert.Equal(t, nodeResourceArgs.DeepCopy(), after.DeepCopy())
}

func TestNodeResourceCapacity(t *testing.T) {
	ctx := context.Background()

	volume := newTestVolume(t)
	nodes := generateNodes(t, volume, 10)
	node := nodes[0]

	_, _, err := volume.GetNodeResourceInfo(ctx, node, nil, false)
	assert.Nil(t, err)

	var after *types.NodeResourceArgs

	nodeResourceOptsDelta := &types.NodeResourceOpts{
		Volumes: types.VolumeMap{"/data4": units.TiB},
		Storage: units.TiB,
		Disks:   types.Disks{},
	}

	nodeResourceArgsDelta := &types.NodeResourceArgs{
		Volumes: types.VolumeMap{"/data4": units.TiB},
		Storage: units.TiB,
		Disks:   types.Disks{},
	}

	originResourceCapacity := &types.NodeResourceArgs{
		Volumes: types.VolumeMap{"/data0": units.TiB, "/data1": units.TiB, "/data2": units.TiB, "/data3": units.TiB},
		Storage: 4 * units.TiB,
		Disks:   types.Disks{},
	}

	originResourceCapacityArgs := &types.NodeResourceArgs{
		Volumes: types.VolumeMap{"/data0": units.TiB, "/data1": units.TiB, "/data2": units.TiB, "/data3": units.TiB},
		Storage: 4 * units.TiB,
		Disks:   types.Disks{},
	}

	afterSetNodeResourceUsageDelta := &types.NodeResourceArgs{
		Volumes: types.VolumeMap{"/data0": units.TiB, "/data1": units.TiB, "/data2": units.TiB, "/data3": units.TiB, "/data4": units.TiB},
		Storage: 5 * units.TiB,
		Disks:   types.Disks{},
	}

	_, after, err = volume.SetNodeResourceCapacity(ctx, node, nodeResourceOptsDelta, nil, true, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDelta)

	_, after, err = volume.SetNodeResourceCapacity(ctx, node, nodeResourceOptsDelta, nil, true, false)
	assert.Nil(t, err)
	assert.Equal(t, after, originResourceCapacity)

	_, after, err = volume.SetNodeResourceCapacity(ctx, node, nil, nodeResourceArgsDelta, true, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDelta)

	_, after, err = volume.SetNodeResourceCapacity(ctx, node, nil, nodeResourceArgsDelta, true, false)
	assert.Nil(t, err)
	assert.Equal(t, after, originResourceCapacity)

	_, after, err = volume.SetNodeResourceCapacity(ctx, node, nil, afterSetNodeResourceUsageDelta, false, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDelta)

	_, after, err = volume.SetNodeResourceCapacity(ctx, node, nil, originResourceCapacityArgs, false, true)
	assert.Nil(t, err)
	assert.Equal(t, after, originResourceCapacity)
}
