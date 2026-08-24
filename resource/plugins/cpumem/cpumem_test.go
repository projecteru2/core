package cpumem

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	enginetypes "github.com/projecteru2/core/engine/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	coretypes "github.com/projecteru2/core/types"
)

func TestName(t *testing.T) {
	cm := initCPUMEM(context.Background(), t)
	assert.Equal(t, cm.name, cm.Name())
}

func initCPUMEM(ctx context.Context, t *testing.T) *Plugin {
	config := coretypes.Config{
		Etcd: coretypes.EtcdConfig{
			Prefix: "/cpumem",
		},
		Scheduler: coretypes.SchedulerConfig{
			MaxShare:  -1,
			ShareBase: 100,
		},
	}

	cluster, err := embedded.New(t.TempDir())
	assert.NoError(t, err)
	t.Cleanup(cluster.Close)
	cm, err := NewPlugin(ctx, config, cluster)
	assert.NoError(t, err)
	return cm
}

func generateNodes(
	ctx context.Context, t *testing.T, cm *Plugin,
	nums, cores int, memory int64, shares, index int,
) []string {
	reqs := generateNodeResourceRequests(t, nums, cores, memory, shares, index)
	info := &enginetypes.Info{NCPU: 8, MemTotal: 2048}
	names := []string{}
	for name, req := range reqs {
		_, err := cm.AddNode(ctx, name, req, info)
		assert.NoError(t, err)
		names = append(names, name)
	}
	t.Cleanup(func() {
		for name := range reqs {
			cm.RemoveNode(ctx, name)
		}
	})
	return names
}

func generateNodeResourceRequests(t *testing.T, nums, cores int, memory int64, shares, index int) map[string]plugintypes.NodeResourceRequest {
	infos := map[string]plugintypes.NodeResourceRequest{}
	for i := index; i < index+nums; i++ {
		info := plugintypes.NodeResourceRequest{
			"cpu":    cores,
			"share":  shares,
			"memory": fmt.Sprintf("%v", memory),
		}
		infos[fmt.Sprintf("test%v", i)] = info
	}
	return infos
}
