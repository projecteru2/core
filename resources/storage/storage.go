package storage

import (
	"github.com/pkg/errors"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

type storageRequest struct {
	request int64
	limit   int64
}

// MakeRequest .
func MakeRequest(opts types.ResourceOptions) (resourcetypes.ResourceRequest, error) {
	sr := &storageRequest{
		request: opts.StorageRequest,
		limit:   opts.StorageLimit,
	}
	return sr, sr.Validate()
}

// Type .
func (s storageRequest) Type() types.ResourceType {
	return types.ResourceStorage
}

// Validate .
func (s *storageRequest) Validate() error {
	if s.limit < 0 || s.request < 0 {
		return errors.Wrap(types.ErrBadStorage, "storage limit or request less than 0")
	}
	if s.limit > 0 && s.request == 0 {
		s.request = s.limit
	}
	if s.limit > 0 && s.request > 0 && s.request > s.limit {
		s.limit = s.request // softlimit storage size
	}
	return nil
}

// MakeScheduler .
func (s storageRequest) MakeScheduler() resourcetypes.SchedulerV2 {
	return func(scheduleInfos []resourcetypes.ScheduleInfo) (plans resourcetypes.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		scheduleInfos, total, err = schedulerV1.SelectStorageNodes(scheduleInfos, s.request)
		return ResourcePlans{
			request:  s.request,
			limit:    s.limit,
			capacity: resourcetypes.GetCapacity(scheduleInfos),
		}, total, err
	}
}

// Rate .
func (s storageRequest) Rate(node types.Node) float64 {
	return float64(s.request) / float64(node.InitStorageCap)
}

// ResourcePlans .
type ResourcePlans struct {
	request  int64
	limit    int64
	capacity map[string]int
}

// Type .
func (rp ResourcePlans) Type() types.ResourceType {
	return types.ResourceStorage
}

// Capacity .
func (rp ResourcePlans) Capacity() map[string]int {
	return rp.capacity
}

// ApplyChangesOnNode .
func (rp ResourcePlans) ApplyChangesOnNode(node *types.Node, indices ...int) {
	node.StorageCap -= int64(len(indices)) * rp.request
}

// RollbackChangesOnNode .
func (rp ResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	node.StorageCap += int64(len(indices)) * rp.request
}

// Dispense .
func (rp ResourcePlans) Dispense(opts resourcetypes.DispenseOptions, r *types.ResourceMeta) (*types.ResourceMeta, error) {
	r.StorageLimit = rp.limit
	r.StorageRequest = rp.request
	return r, nil
}
