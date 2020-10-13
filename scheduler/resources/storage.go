package resources

import (
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

type StorageResourceRequest struct {
	Quota int64
}

func (r StorageResourceRequest) Type() types.ResourceType {
	return types.ResourceStorage
}

func (r StorageResourceRequest) DeployValidate() error { return nil }

func (r StorageResourceRequest) MakeScheduler() types.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans types.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		nodesInfo, total, err = schedulerV1.SelectStorageNodes(nodesInfo, r.Quota)
		return StorageResourcePlans{
			quota:    r.Quota,
			capacity: getCapacity(nodesInfo),
		}, total, err
	}
}

func (r StorageResourceRequest) Rate(node types.Node) float64 {
	return float64(0) / float64(node.Volume.Total())
}

type StorageResourcePlans struct {
	quota    int64
	capacity map[string]int
}

func (p StorageResourcePlans) Type() types.ResourceType {
	return types.ResourceStorage
}

func (p StorageResourcePlans) Capacity() map[string]int {
	return p.capacity
}

func (p StorageResourcePlans) ApplyChangesOnNode(node *types.Node, indices ...int) {
	node.StorageCap -= int64(len(indices)) * p.quota
}

func (p StorageResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	node.StorageCap += int64(len(indices)) * p.quota
}

func (p StorageResourcePlans) Dispense(opts types.DispenseOptions, resources *types.Resources) error {
	resources.Storage = p.quota
	return nil
}
