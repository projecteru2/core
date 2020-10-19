package resources

import (
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

// StorageResourceRequest .
type StorageResourceRequest struct {
	Quota int64
}

// Type .
func (r StorageResourceRequest) Type() types.ResourceType {
	return types.ResourceStorage
}

// DeployValidate .
func (r StorageResourceRequest) DeployValidate() error { return nil }

// MakeScheduler .
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

// Rate .
func (r StorageResourceRequest) Rate(node types.Node) float64 {
	return float64(0) / float64(node.Volume.Total())
}

// StorageResourcePlans .
type StorageResourcePlans struct {
	quota    int64
	capacity map[string]int
}

// Type .
func (p StorageResourcePlans) Type() types.ResourceType {
	return types.ResourceStorage
}

// Capacity .
func (p StorageResourcePlans) Capacity() map[string]int {
	return p.capacity
}

// ApplyChangesOnNode .
func (p StorageResourcePlans) ApplyChangesOnNode(node *types.Node, indices ...int) {
	node.StorageCap -= int64(len(indices)) * p.quota
}

// RollbackChangesOnNode .
func (p StorageResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	node.StorageCap += int64(len(indices)) * p.quota
}

// Dispense .
func (p StorageResourcePlans) Dispense(opts types.DispenseOptions, resources *types.Resources) error {
	resources.Storage = p.quota
	return nil
}
