package storage

import (
	"testing"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	schedulerMocks "github.com/projecteru2/core/scheduler/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMakeRequest(t *testing.T) {
	_, err := MakeRequest(types.ResourceOptions{
		StorageRequest: -1,
		StorageLimit:   -1,
	})
	assert.NotNil(t, err)

	_, err = MakeRequest(types.ResourceOptions{
		StorageRequest: 1024,
		StorageLimit:   1024,
	})
	assert.Nil(t, err)

	_, err = MakeRequest(types.ResourceOptions{
		StorageRequest: 0,
		StorageLimit:   1024,
	})
	assert.Nil(t, err)

	_, err = MakeRequest(types.ResourceOptions{
		StorageRequest: 1024,
		StorageLimit:   0,
	})
	assert.Nil(t, err)

	_, err = MakeRequest(types.ResourceOptions{
		StorageRequest: 2024,
		StorageLimit:   1024,
	})
	assert.Nil(t, err)
}

func TestRate(t *testing.T) {
	req, err := MakeRequest(types.ResourceOptions{
		StorageRequest: 1024,
		StorageLimit:   1024,
	})
	assert.Nil(t, err)
	node := types.Node{
		InitStorageCap: 1024,
	}
	assert.Equal(t, req.Rate(node), 1.0)
}

func TestStorage(t *testing.T) {
	mockScheduler := &schedulerMocks.Scheduler{}
	var (
		nodeInfos []types.NodeInfo = []types.NodeInfo{
			{
				Name:       "TestNode",
				CPUMap:     map[string]int64{"0": 10000, "1": 10000},
				NUMA:       map[string]string{"0": "0", "1": "1"},
				NUMAMemory: map[string]int64{"0": 1024, "1": 1204},
				MemCap:     10240,
				CPUPlan:    []types.CPUMap{{"0": 10000, "1": 10000}},
				StorageCap: 10240,
			},
		}
	)
	mockScheduler.On(
		"SelectStorageNodes", mock.Anything, mock.Anything,
	).Return(nodeInfos, 1, nil)

	resourceRequest, err := MakeRequest(types.ResourceOptions{
		StorageRequest: 1024,
		StorageLimit:   1024,
	})
	assert.NoError(t, err)
	_, _, err = resourceRequest.MakeScheduler()([]types.NodeInfo{})
	assert.Error(t, err)

	assert.True(t, resourceRequest.Type()&types.ResourceStorage > 0)
	prevSche, _ := scheduler.GetSchedulerV1()
	scheduler.InitSchedulerV1(mockScheduler)
	defer func() {
		scheduler.InitSchedulerV1(prevSche)
	}()

	sche := resourceRequest.MakeScheduler()
	plans, _, err := sche(nodeInfos)
	assert.Nil(t, err)

	const storage = int64(10240)
	var node = types.Node{
		Name:       "TestNode",
		CPU:        map[string]int64{"0": 10000, "1": 10000},
		NUMA:       map[string]string{"0": "0", "1": "1"},
		NUMAMemory: map[string]int64{"0": 1024, "1": 1204},
		MemCap:     10240,
		StorageCap: storage,
	}

	assert.True(t, plans.Type()&types.ResourceStorage > 0)

	assert.NotNil(t, plans.Capacity())

	plans.ApplyChangesOnNode(&node, 0)
	assert.Less(t, node.StorageCap, storage)

	plans.RollbackChangesOnNode(&node, 0)
	assert.Equal(t, node.StorageCap, storage)

	opts := resourcetypes.DispenseOptions{
		Node:  &node,
		Index: 0,
	}
	r := &types.ResourceMeta{}
	_, err = plans.Dispense(opts, r)
	assert.Nil(t, err)
}
