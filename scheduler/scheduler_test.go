package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/scheduler/complex"
	"gitlab.ricebook.net/platform/core/types"
)

func TestSchedulerInvoke(t *testing.T) {
	coreCfg := types.Config{
		EtcdMachines:   []string{"http://127.0.0.1:2379"},
		EtcdLockPrefix: "/eru-core/_lock",
		Scheduler: types.SchedConfig{
			LockKey: "/coretest",
			LockTTL: 1,
			Type:    "complex",
		},
	}
	scheduler, _ := complexscheduler.New(coreCfg)

	nodes := []types.NodeInfo{
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 10,
				},
				12400000,
			},
			"node1", 0.0, 0, 0, 0,
		},
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 10,
				},
				12400000,
			},
			"node2", 0.0, 0, 0, 0,
		},
	}
	_, _, err := scheduler.SelectCPUNodes(nodes, 1, 2)
	assert.NoError(t, err)
}
