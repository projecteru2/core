package scheduler

import (
	"testing"

	"github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/scheduler/complex"
	"gitlab.ricebook.net/platform/core/types"
)

func TestSchedulerInvoke(t *testing.T) {
	// scheduler := complexscheduler.New()
	cfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}
	cli, _ := client.New(cfg)
	api := client.NewKeysAPI(cli)

	scheduler, _ := complexscheduler.NewPotassim(api, "/coretest", 1)

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

	_, err := scheduler.SelectNodes(nodes, 1, 2)
	assert.NoError(t, err)
}
