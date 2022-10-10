package models

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resources/volume/types"
	"github.com/projecteru2/core/store/etcdv3/meta"
	coretypes "github.com/projecteru2/core/types"
)

func newTestVolume(t *testing.T) *Volume {
	config := coretypes.Config{
		Etcd: coretypes.EtcdConfig{
			Prefix: "/Volume",
		},
		Scheduler: coretypes.SchedulerConfig{
			MaxShare:       -1,
			ShareBase:      100,
			MaxDeployCount: 1000,
		},
	}
	volume, err := NewVolume(config)
	assert.Nil(t, err)
	store, err := meta.NewETCD(config.Etcd, t)
	assert.Nil(t, err)
	volume.store = store
	return volume
}

func generateNodeResourceInfos(num int) []*types.NodeResourceInfo {
	var infos []*types.NodeResourceInfo
	for i := 0; i < num; i++ {
		info := &types.NodeResourceInfo{
			Capacity: &types.NodeResourceArgs{
				Volumes: types.VolumeMap{
					"/data0": units.TiB,
					"/data1": units.TiB,
					"/data2": units.TiB,
					"/data3": units.TiB,
				},
				Storage: 4 * units.TiB,
			},
			Usage: &types.NodeResourceArgs{
				Volumes: types.VolumeMap{
					"/data0": 200 * units.GiB,
					"/data1": 300 * units.GiB,
				},
				Storage: 500 * units.GiB,
			},
		}
		infos = append(infos, info)
	}
	return infos
}

func generateNodes(t *testing.T, volume *Volume, num int) []string {
	nodes := []string{}
	infos := generateNodeResourceInfos(num)

	for i, info := range infos {
		nodename := fmt.Sprintf("node%d", i)
		assert.Nil(t, volume.doSetNodeResourceInfo(context.Background(), nodename, info))
		nodes = append(nodes, nodename)
	}

	return nodes
}

func generateVolumeBindings(t *testing.T, str []string) types.VolumeBindings {
	bindings, err := types.NewVolumeBindings(str)
	assert.Nil(t, err)
	return bindings
}
