package storage

import (
	"github.com/pkg/errors"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

// storageResourceRequirement .
type storageResourceRequirement struct {
	request int64
	limit   int64
}

// NewResourceRequirement .
func NewResourceRequirement(opts types.RawResourceOptions) (resourcetypes.ResourceRequirement, error) {
	a := &storageResourceRequirement{
		request: opts.StorageRequest,
		limit:   opts.StorageLimit,
	}
	return a, a.Validate()
}

// Type .
func (a storageResourceRequirement) Type() types.ResourceType {
	return types.ResourceStorage
}

// Validate .
func (a *storageResourceRequirement) Validate() error {
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
func (a storageResourceRequirement) MakeScheduler() resourcetypes.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans resourcetypes.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		nodesInfo, total, err = schedulerV1.SelectStorageNodes(nodesInfo, a.request)
		return ResourcePlans{
			request:  a.request,
			limit:    a.limit,
			capacity: resourcetypes.GetCapacity(nodesInfo),
		}, total, err
	}
}

// Rate .
func (a storageResourceRequirement) Rate(node types.Node) float64 {
	return float64(0) / float64(node.Volume.Total())
}

// ResourcePlans .
type ResourcePlans struct {
	request  int64
	limit    int64
	capacity map[string]int
}

// Type .
func (p ResourcePlans) Type() types.ResourceType {
	return types.ResourceStorage
}

// Capacity .
func (p ResourcePlans) Capacity() map[string]int {
	return p.capacity
}

// ApplyChangesOnNode .
func (p ResourcePlans) ApplyChangesOnNode(node *types.Node, indices ...int) {
	node.StorageCap -= int64(len(indices)) * p.request
}

// RollbackChangesOnNode .
func (p ResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	node.StorageCap += int64(len(indices)) * p.request
}

// Dispense .
func (p ResourcePlans) Dispense(opts resourcetypes.DispenseOptions, rsc *types.Resources) error {
	rsc.StorageLimit = p.limit
	rsc.StorageRequest = p.request
	return nil
}
