package resources

import (
	"github.com/pkg/errors"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

// StorageResourceApply .
type StorageResourceApply struct {
	request int64
	limit   int64
}

func NewStorageApplication(request, limit int64) (*StorageResourceApply, error) {
	apply := &StorageResourceApply{
		request: request,
		limit:   limit,
	}
	return apply, apply.Validate()
}

// Type .
func (a StorageResourceApply) Type() types.ResourceType {
	return types.ResourceStorage
}

// Validate .
func (a StorageResourceApply) Validate() error {
	if a.limit > 0 && a.request == 0 {
		a.request = a.limit
	}
	if a.limit < 0 || a.request < 0 {
		return errors.Wrap(types.ErrBadStorage, "storage limit or request less than 0")
	}
	if a.limit > 0 && a.request > 0 && a.request > a.limit {
		return errors.Wrap(types.ErrBadStorage, "storage limit less than request")
	}
	return nil
}

// MakeScheduler .
func (a StorageResourceApply) MakeScheduler() types.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans types.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		nodesInfo, total, err = schedulerV1.SelectStorageNodes(nodesInfo, a.request)
		return StorageResourcePlans{
			request:  a.request,
			limit:    a.limit,
			capacity: getCapacity(nodesInfo),
		}, total, err
	}
}

// Rate .
func (a StorageResourceApply) Rate(node types.Node) float64 {
	return float64(0) / float64(node.Volume.Total())
}

// StorageResourcePlans .
type StorageResourcePlans struct {
	request  int64
	limit    int64
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
	node.StorageCap -= int64(len(indices)) * p.request
}

// RollbackChangesOnNode .
func (p StorageResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	node.StorageCap += int64(len(indices)) * p.request
}

// Dispense .
func (p StorageResourcePlans) Dispense(opts types.DispenseOptions, resources *types.Resources) error {
	resources.Storage = p.limit
	return nil
}
