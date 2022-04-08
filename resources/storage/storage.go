package storage

import (
	"context"

	"github.com/pkg/errors"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
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
	if sr.limit > 0 && sr.request == 0 {
		sr.request = sr.limit
	}
	// add volume request / limit to storage request / limit
	if len(opts.VolumeRequest) != 0 && len(opts.VolumeLimit) != len(opts.VolumeRequest) {
		return nil, errors.Wrapf(types.ErrBadVolume, "volume request and limit must be the same length")
	}
	for idx := range opts.VolumeLimit {
		if len(opts.VolumeRequest) > 0 {
			sr.request += opts.VolumeRequest[idx].SizeInBytes
			sr.limit += utils.Max(opts.VolumeLimit[idx].SizeInBytes, opts.VolumeRequest[idx].SizeInBytes)
		} else {
			sr.request += opts.VolumeLimit[idx].SizeInBytes
			sr.limit += opts.VolumeLimit[idx].SizeInBytes
		}
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
	return func(ctx context.Context, scheduleInfos []resourcetypes.ScheduleInfo) (plans resourcetypes.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		scheduleInfos, total, err = schedulerV1.SelectStorageNodes(ctx, scheduleInfos, s.request)
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
	if rp.capacity[opts.Node.Name] <= opts.Index {
		return nil, errors.WithStack(types.ErrInsufficientCap)
	}
	r.StorageLimit = rp.limit
	r.StorageRequest = rp.request
	return r, nil
}
