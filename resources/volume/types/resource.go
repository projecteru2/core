package types

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/errors"

	coretypes "github.com/projecteru2/core/types"
	coreutils "github.com/projecteru2/core/utils"
)

// WorkloadResourceOpts .
type WorkloadResourceOpts struct {
	VolumesRequest VolumeBindings `json:"volumes_request"`
	VolumesLimit   VolumeBindings `json:"volumes_limit"`

	StorageRequest int64 `json:"storage_request"`
	StorageLimit   int64 `json:"storage_limit"`

	once sync.Once
}

// SkipAddStorage will skip adding volume size to storage request / limit (used in realloc)
func (w *WorkloadResourceOpts) SkipAddStorage() {
	w.once.Do(func() {})
}

func (w *WorkloadResourceOpts) validateVolumes() error {
	if len(w.VolumesLimit) > 0 && len(w.VolumesRequest) == 0 {
		w.VolumesRequest = w.VolumesLimit
	}
	if len(w.VolumesRequest) != len(w.VolumesLimit) {
		return errors.Wrap(ErrInvalidVolume, "different length of request and limit")
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
			return errors.Wrap(ErrInvalidVolume, err.Error())
		}
	}
	return nil
}

func (w *WorkloadResourceOpts) validateStorage() error {
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

// Validate .
func (w *WorkloadResourceOpts) Validate() error {
	if err := w.validateVolumes(); err != nil {
		return err
	}
	if err := w.validateStorage(); err != nil {
		return err
	}
	return nil
}

// ParseFromRawParams .
func (w *WorkloadResourceOpts) ParseFromRawParams(rawParams coretypes.RawParams) (err error) {
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

// WorkloadResourceArgs .
type WorkloadResourceArgs struct {
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
func (w *WorkloadResourceArgs) ParseFromRawParams(rawParams coretypes.RawParams) (err error) {
	body, err := json.Marshal(rawParams)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, w)
}

// NodeResourceOpts .
type NodeResourceOpts struct {
	Volumes VolumeMap `json:"volumes"`
	Storage int64     `json:"storage"`
	Disks   Disks     `json:"disks"`

	RawParams coretypes.RawParams `json:"-"`
}

// ParseFromRawParams .
func (n *NodeResourceOpts) ParseFromRawParams(rawParams coretypes.RawParams) (err error) {
	n.RawParams = rawParams

	volumes := VolumeMap{}
	for _, volume := range n.RawParams.StringSlice("volumes") {
		parts := strings.Split(volume, ":")
		if len(parts) != 2 {
			return errors.Wrap(ErrInvalidVolume, "volume should have 2 parts")
		}

		capacity, err := coreutils.ParseRAMInHuman(parts[1])
		if err != nil {
			return err
		}
		volumes[parts[0]] = capacity
	}
	n.Volumes = volumes

	if n.Storage, err = coreutils.ParseRAMInHuman(n.RawParams.String("storage")); err != nil {
		return err
	}

	if n.RawParams.IsSet("disks") {
		for _, rawDiskStr := range n.RawParams.StringSlice("disks") {
			disk := &Disk{}
			if err = disk.ParseFromString(rawDiskStr); err != nil {
				return errors.Wrapf(ErrInvalidDisk, "wrong disk format: %v, %v", rawDiskStr, err)
			}
			n.Disks = append(n.Disks, disk)
		}
	}

	return nil
}

// SkipEmpty used for setting node resource capacity in absolute mode
func (n *NodeResourceOpts) SkipEmpty(resourceCapacity *NodeResourceArgs) {
	if n == nil {
		return
	}
	if !n.RawParams.IsSet("volumes") {
		n.Volumes = resourceCapacity.Volumes
	}
	if !n.RawParams.IsSet("storage") {
		n.Storage = resourceCapacity.Storage
	}
	if !n.RawParams.IsSet("disks") {
		n.Disks = resourceCapacity.Disks
	}
}

// NodeResourceArgs .
type NodeResourceArgs struct {
	Volumes VolumeMap `json:"volumes"`
	Storage int64     `json:"storage"`
	Disks   Disks     `json:"disks"`
}

// ParseFromRawParams .
func (n *NodeResourceArgs) ParseFromRawParams(rawParams coretypes.RawParams) error {
	body, err := json.Marshal(rawParams)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, n)
}

// DeepCopy .
func (n *NodeResourceArgs) DeepCopy() *NodeResourceArgs {
	return &NodeResourceArgs{Volumes: n.Volumes.DeepCopy(), Storage: n.Storage, Disks: n.Disks.DeepCopy()}
}

// RemoveEmpty .
func (n *NodeResourceArgs) RemoveEmpty() {
	for device, size := range n.Volumes {
		if n.Volumes[device] == 0 {
			n.Storage -= size
			delete(n.Volumes, device)
		}
	}
}

// Add .
func (n *NodeResourceArgs) Add(n1 *NodeResourceArgs) {
	for k, v := range n1.Volumes {
		n.Volumes[k] += v
	}
	n.Storage += n1.Storage
	n.Disks.Add(n1.Disks)
}

// Sub .
func (n *NodeResourceArgs) Sub(n1 *NodeResourceArgs) {
	for k, v := range n1.Volumes {
		n.Volumes[k] -= v
	}
	n.Storage -= n1.Storage
	n.Disks.Sub(n1.Disks)
}

// NodeResourceInfo .
type NodeResourceInfo struct {
	Capacity *NodeResourceArgs `json:"capacity"`
	Usage    *NodeResourceArgs `json:"usage"`
}

func (n *NodeResourceInfo) validateDisks() error {
	for _, disk := range n.Capacity.Disks {
		if disk.ReadIOPS < 0 || disk.WriteIOPS < 0 || disk.ReadBPS < 0 || disk.WriteBPS < 0 {
			return errors.Wrap(ErrInvalidDisk, "disk IOPS / BPS can't be negative")
		}

		usage := n.Usage.Disks.GetDiskByDevice(disk.Device)
		if usage == nil {
			continue
		}
		if usage.ReadIOPS < 0 || usage.WriteIOPS < 0 || usage.ReadBPS < 0 || usage.WriteBPS < 0 {
			return errors.Wrap(ErrInvalidDisk, "disk IOPS / BPS can't be negative")
		}
		if usage.ReadIOPS > disk.ReadIOPS || usage.WriteIOPS > disk.WriteIOPS || usage.ReadBPS > disk.ReadBPS || usage.WriteBPS > disk.WriteBPS {
			return errors.Wrap(ErrInvalidDisk, "disk IOPS / BPS usage can't be greater than capacity")
		}
	}

	for _, disk := range n.Capacity.Disks {
		if disk.ReadIOPS < 0 || disk.WriteIOPS < 0 || disk.ReadBPS < 0 || disk.WriteBPS < 0 {
			return errors.Wrap(ErrInvalidDisk, "disk IOPS / BPS can't be negative")
		}
	}
	return nil
}

func (n *NodeResourceInfo) validateVolume() error {
	for key, value := range n.Capacity.Volumes {
		if value < 0 {
			return errors.Wrap(ErrInvalidVolume, "volume size should not be less than 0")
		}
		if usage, ok := n.Usage.Volumes[key]; ok && (usage > value || usage < 0) {
			return errors.Wrap(ErrInvalidVolume, "invalid size in usage")
		}
	}
	for key := range n.Usage.Volumes {
		if _, ok := n.Usage.Volumes[key]; !ok {
			return errors.Wrap(ErrInvalidVolume, "invalid key in usage")
		}
	}
	return nil
}

func (n *NodeResourceInfo) validateStorage() error {
	if n.Capacity.Storage < 0 {
		return errors.Wrap(ErrInvalidStorage, "storage capacity can't be negative")
	}
	if n.Usage.Storage < 0 {
		return errors.Wrap(ErrInvalidStorage, "storage usage can't be negative")
	}
	return nil
}

// Validate .
func (n *NodeResourceInfo) Validate() error {
	if n.Capacity == nil {
		return ErrInvalidCapacity
	}
	if n.Usage == nil {
		n.Usage = &NodeResourceArgs{Volumes: VolumeMap{}, Storage: 0}
		for device := range n.Capacity.Volumes {
			n.Usage.Volumes[device] = 0
		}
		for _, disk := range n.Capacity.Disks {
			n.Usage.Disks = append(n.Usage.Disks, &Disk{
				Device:    disk.Device,
				Mounts:    disk.Mounts,
				ReadIOPS:  0,
				WriteIOPS: 0,
				ReadBPS:   0,
				WriteBPS:  0,
			})
		}
	}

	// sort disks
	sort.Slice(n.Usage.Disks, func(i, j int) bool {
		return n.Usage.Disks[i].Device < n.Usage.Disks[j].Device
	})
	sort.Slice(n.Capacity.Disks, func(i, j int) bool {
		return n.Capacity.Disks[i].Device < n.Capacity.Disks[j].Device
	})

	if err := n.validateVolume(); err != nil {
		return err
	}
	if err := n.validateStorage(); err != nil {
		return err
	}
	if err := n.validateDisks(); err != nil {
		return err
	}
	return nil
}

// GetAvailableResource .
func (n *NodeResourceInfo) GetAvailableResource() *NodeResourceArgs {
	res := n.Capacity.DeepCopy()
	res.Sub(n.Usage)
	return res
}

// NodeCapacityInfo .
type NodeCapacityInfo struct {
	Node     string  `json:"node"`
	Capacity int     `json:"capacity"`
	Usage    float64 `json:"usage"`
	Rate     float64 `json:"rate"`
	Weight   int     `json:"weight"`
}

// EngineArgs .
type EngineArgs struct {
	Volumes       []string          `json:"volumes"`
	VolumeChanged bool              `json:"volume_changed"` // indicates whether the realloc request includes new volumes
	Storage       int64             `json:"storage"`
	IOPSOptions   map[string]string `json:"iops_options"`
}

// WorkloadResourceArgsMap .
type WorkloadResourceArgsMap map[string]*WorkloadResourceArgs

// ParseFromRawParamsMap .
func (w *WorkloadResourceArgsMap) ParseFromRawParamsMap(rawParamsMap map[string]coretypes.RawParams) error {
	body, err := json.Marshal(rawParamsMap)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, w)
}
