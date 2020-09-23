package resources

import (
	complexscheduler "github.com/projecteru2/core/scheduler/complex"
	"github.com/projecteru2/core/types"
)

type StorageResourceRequest types.StorageResourceRequest

func (r StorageResourceRequest) MakeScheduler() types.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans StorageResourcePlans, total int, err error) {
		schedulerV1, err := complexscheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		nodesInfo, total, err = schedulerV1.SelectStorageNodes(nodesInfo, r.Quota)
		return StorageResourcePlans{r.Quota}, total, err
	}
}

type StorageResourcePlans struct {
	quota int64
}

func (p StorageResourcePlans) Type() types.ResourceType {
	return types.ResourceStorage
}

func (p StorageResourcePlans) ApplyChangesOnNode(nodeInfo types.NodeInfo, node *types.Node) (err error) {
	node.StorageCap -= int64(nodeInfo.Deploy) * p.quota
	return
}
