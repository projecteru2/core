package models

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resources/cpumem/types"
	"github.com/projecteru2/core/store/etcdv3/meta"
	coretypes "github.com/projecteru2/core/types"
)

func newTestCPUMem(t *testing.T) *CPUMem {
	config := coretypes.Config{
		Etcd: coretypes.EtcdConfig{
			Prefix: "/cpumem",
		},
		Scheduler: coretypes.SchedConfig{
			MaxShare:  -1,
			ShareBase: 100,
		},
	}
	cpuMem, err := NewCPUMem(config)
	assert.Nil(t, err)
	store, err := meta.NewETCD(config.Etcd, t)
	assert.Nil(t, err)
	cpuMem.store = store
	return cpuMem
}

func generateNodeResourceInfos(t *testing.T, nums int, cores int, memory int64, shares int) []*types.NodeResourceInfo {
	infos := []*types.NodeResourceInfo{}
	for i := 0; i < nums; i++ {
		cpuMap := types.CPUMap{}
		for c := 0; c < cores; c++ {
			cpuMap[strconv.Itoa(c)] = shares
		}

		info := &types.NodeResourceInfo{
			Capacity: &types.NodeResourceArgs{
				CPU:    float64(cores),
				CPUMap: cpuMap,
				Memory: memory,
			},
			Usage: nil,
		}
		assert.Nil(t, info.Validate())

		infos = append(infos, info)
	}
	return infos
}

func generateNodes(t *testing.T, cpuMem *CPUMem, nums int, cores int, memory int64, shares int) []string {
	nodes := []string{}
	infos := generateNodeResourceInfos(t, nums, cores, memory, shares)

	for i, info := range infos {
		nodeName := fmt.Sprintf("node%d", i)
		assert.Nil(t, cpuMem.doSetNodeResourceInfo(context.Background(), nodeName, info))
		nodes = append(nodes, nodeName)
	}

	return nodes
}

func TestNewCPUMem(t *testing.T) {
	config := coretypes.Config{
		Etcd: coretypes.EtcdConfig{
			Machines: []string{"invalid-address"},
		},
		Scheduler: coretypes.SchedConfig{
			MaxShare:  -1,
			ShareBase: 100,
		},
	}
	cpuMem, err := NewCPUMem(config)
	assert.Nil(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = cpuMem.store.Put(ctx, "/test", "test")
	assert.NotNil(t, err)
}
