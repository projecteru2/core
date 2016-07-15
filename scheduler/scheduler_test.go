package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/scheduler/complex"
	"gitlab.ricebook.net/platform/core/types"
)

func TestSchedulerInvoke(t *testing.T) {
	// scheduler := complexscheduler.New()
	coreCfg := types.Config{
		EtcdMachines: []string{"http://127.0.0.1:2379"},
		Scheduler: types.SchedConfig{
			EtcdLockKey: "/coretest",
			EtcdLockTTL: 1,
			Type:        "complex",
		},
	}
	scheduler, _ := complexscheduler.NewPotassim(coreCfg)

	nodes := map[string]types.CPUMap{
		"node1": types.CPUMap{
			"0": 10,
			"1": 10,
		},
		"node2": types.CPUMap{
			"0": 10,
			"1": 10,
		},
	}

	_, _, err := scheduler.SelectNodes(nodes, 1, 2)
	assert.NoError(t, err)
}
