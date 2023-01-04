package types

import (
	"sort"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/mitchellh/mapstructure"
	coretypes "github.com/projecteru2/core/types"
	coreutils "github.com/projecteru2/core/utils"
)

// WorkloadResource .
type WorkloadResource struct {
	VolumesRequest VolumeBindings `json:"volumes_request"`
	VolumesLimit   VolumeBindings `json:"volumes_limit"`

	VolumePlanRequest VolumePlan `json:"volume_plan_request"`
	VolumePlanLimit   VolumePlan `json:"volume_plan_limit"`

	StorageRequest int64 `json:"storage_request"`
	StorageLimit   int64 `json:"storage_limit"`

	DisksRequest Disks `json:"disks_request"`
	DisksLimit   Disks `json:"disks_limit"`
}

// ParseFromRawParams .
func (w *WorkloadResource) Parse(rawParams *coretypes.RawParams) error {
	return mapstructure.Decode(rawParams, w)
}

// WorkloadResourceRequest .
type WorkloadResourceRequest struct {
	VolumesRequest VolumeBindings `json:"volumes_request"`
	VolumesLimit   VolumeBindings `json:"volumes_limit"`
	StorageRequest int64          `json:"storage_request"`
	StorageLimit   int64          `json:"storage_limit"`

	once sync.Once
}

// Validate .
func (w *WorkloadResourceRequest) Validate() error {
	return errors.CombineErrors(
		w.validateVolumes(),
		w.validateStorage(),
	)
}

// ParseFromRawParams .
func (w *WorkloadResourceRequest) Parse(rawParams *coretypes.RawParams) (err error) {
	if w.VolumesRequest, err = NewVolumeBindings(rawParams.OneOfStringSlice("volumes-request", "volume-request", "volumes-request")); err != nil {
		return err
	}
	if w.VolumesLimit, err = NewVolumeBindings(rawParams.OneOfStringSlice("volumes", "volume", "volume-limit", "volumes-limit")); err != nil {
		return err
	}

	if w.StorageRequest, err = coreutils.ParseRAMInHuman(rawParams.String("storage-request")); err != nil {
		return err
	}
	if w.StorageLimit, err = coreutils.ParseRAMInHuman(rawParams.String("storage-limit")); err != nil {
		return err
	}
	if rawParams.IsSet("storage") {
		storage, err := coreutils.ParseRAMInHuman(rawParams.String("storage"))
		if err != nil {
			return err
		}
		w.StorageLimit = storage
		w.StorageRequest = storage
	}
	return nil
}

// SkipAddStorage will skip adding volume size to storage request / limit (used in realloc)
func (w *WorkloadResourceRequest) SkipAddStorage() {
	w.once.Do(func() {})
}

func (w *WorkloadResourceRequest) validateVolumes() error {
	if len(w.VolumesLimit) > 0 && len(w.VolumesRequest) == 0 {
		w.VolumesRequest = w.VolumesLimit
	}
	if len(w.VolumesRequest) != len(w.VolumesLimit) {
		return errors.Wrap(ErrInvalidVolume, "different length of request and limit")
	}
	if err := w.VolumesRequest.Validate(); err != nil {
		return errors.CombineErrors(ErrInvalidVolume, err)
	}
	if err := w.VolumesLimit.Validate(); err != nil {
		return errors.CombineErrors(ErrInvalidVolume, err)
	}

	sortFunc := func(volumeBindings []*VolumeBinding) func(i, j int) bool {
		return func(i, j int) bool {
			return volumeBindings[i].ToString(false) < volumeBindings[j].ToString(false)
		}
	}

	sort.Slice(w.VolumesRequest, sortFunc(w.VolumesRequest))
	sort.Slice(w.VolumesLimit, sortFunc(w.VolumesLimit))

	for i := range w.VolumesRequest {
		request := w.VolumesRequest[i]
		limit := w.VolumesLimit[i]
		if request.Source != limit.Source || request.Destination != limit.Destination || request.Flags != limit.Flags {
			return errors.Wrap(ErrInvalidVolume, "request and limit not match")
		}
		if request.SizeInBytes > 0 && limit.SizeInBytes > 0 && request.SizeInBytes > limit.SizeInBytes {
			limit.SizeInBytes = request.SizeInBytes
		}
		if request.ReadIOPS > limit.ReadIOPS {
			limit.ReadIOPS = request.ReadIOPS
		}
		if request.WriteIOPS > limit.WriteIOPS {
			limit.WriteIOPS = request.WriteIOPS
		}
		if request.ReadBPS > limit.ReadBPS {
			limit.ReadBPS = request.ReadBPS
		}
		if request.WriteBPS > limit.WriteBPS {
			limit.WriteBPS = request.WriteBPS
		}
	}

	for _, vb := range append(w.VolumesRequest, w.VolumesLimit...) {
		if err := vb.Validate(); err != nil {
			return errors.CombineErrors(ErrInvalidVolume, err)
		}
	}
	return nil
}

func (w *WorkloadResourceRequest) validateStorage() error {
	if w.StorageLimit < 0 || w.StorageRequest < 0 {
		return errors.Wrap(ErrInvalidStorage, "storage limit or request less than 0")
	}
	if w.StorageLimit > 0 && w.StorageRequest == 0 {
		w.StorageRequest = w.StorageLimit
	}
	if w.StorageLimit > 0 && w.StorageRequest > 0 && w.StorageRequest > w.StorageLimit {
		w.StorageLimit = w.StorageRequest // soft limit storage size
	}

	// ensure to change storage request / limit only once
	w.once.Do(func() {
		w.StorageRequest += w.VolumesRequest.TotalSize()
		w.StorageLimit += w.VolumesLimit.TotalSize()
	})
	return nil
}
