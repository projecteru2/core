package types

import (
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/mitchellh/mapstructure"
	coretypes "github.com/projecteru2/core/types"
	coreutils "github.com/projecteru2/core/utils"
)

// NodeResource .
type NodeResource struct {
	Volumes VolumeMap `json:"volumes"`
	Disks   Disks     `json:"disks"`
	Storage int64     `json:"storage"`
}

// ParseFromRawParams .
func (n *NodeResource) Parse(rawParams *coretypes.RawParams) error {
	return mapstructure.Decode(rawParams, n)
}

// DeepCopy .
func (n *NodeResource) DeepCopy() *NodeResource {
	return &NodeResource{Volumes: n.Volumes.DeepCopy(), Storage: n.Storage, Disks: n.Disks.DeepCopy()}
}

// RemoveEmpty .
func (n *NodeResource) RemoveEmpty() {
	for device, size := range n.Volumes {
		if n.Volumes[device] == 0 {
			n.Storage -= size
			delete(n.Volumes, device)
		}
	}
}

// Add .
func (n *NodeResource) Add(n1 *NodeResource) {
	for k, v := range n1.Volumes {
		n.Volumes[k] += v
	}
	n.Storage += n1.Storage
	n.Disks.Add(n1.Disks)
}

// Sub .
func (n *NodeResource) Sub(n1 *NodeResource) {
	for k, v := range n1.Volumes {
		n.Volumes[k] -= v
	}
	n.Storage -= n1.Storage
	n.Disks.Sub(n1.Disks)
}

// NodeResourceInfo .
type NodeResourceInfo struct {
	Capacity *NodeResource `json:"capacity"`
	Usage    *NodeResource `json:"usage"`
}

// Validate .
func (n *NodeResourceInfo) Validate() error {
	if n.Capacity == nil {
		return ErrInvalidCapacity
	}
	if n.Usage == nil {
		n.Usage = &NodeResource{Volumes: VolumeMap{}, Storage: 0}
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
func (n *NodeResourceInfo) GetAvailableResource() *NodeResource {
	res := n.Capacity.DeepCopy()
	res.Sub(n.Usage)
	return res
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
		usage.Mounts = disk.Mounts
		if usage.ReadIOPS < 0 || usage.WriteIOPS < 0 || usage.ReadBPS < 0 || usage.WriteBPS < 0 {
			return errors.Wrap(ErrInvalidDisk, "disk IOPS / BPS can't be negative")
		}
		if usage.ReadIOPS > disk.ReadIOPS || usage.WriteIOPS > disk.WriteIOPS || usage.ReadBPS > disk.ReadBPS || usage.WriteBPS > disk.WriteBPS {
			return errors.Wrap(ErrInvalidDisk, "disk IOPS / BPS usage can't be greater than capacity")
		}
	}

	toRemoveMap := map[string]struct{}{}
	for _, disk := range n.Usage.Disks {
		capacity := n.Capacity.Disks.GetDiskByDevice(disk.Device)
		if capacity == nil {
			if disk.ReadIOPS == 0 && disk.WriteIOPS == 0 && disk.ReadBPS == 0 && disk.WriteBPS == 0 {
				toRemoveMap[disk.Device] = struct{}{}
			} else {
				return errors.Wrapf(ErrInvalidDisk, "disk %+v not found in capacity", disk.Device)
			}
		}

	}
	n.Usage.Disks = coreutils.Filter(n.Usage.Disks, func(disk *Disk) bool {
		_, ok := toRemoveMap[disk.Device]
		return !ok
	})
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

// NodeResourceRequest includes all possible fields passed by eru-core for editing node, it not parsed!
type NodeResourceRequest struct {
	Volumes VolumeMap `json:"volumes"`
	Storage int64     `json:"storage"`
	Disks   Disks     `json:"disks"`
	RMDisks []string  `json:"rm_disks"`

	RawParams *coretypes.RawParams `json:"-"`
}

// Parse .
func (n *NodeResourceRequest) Parse(rawParams *coretypes.RawParams) (err error) {
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

	n.Storage += n.Volumes.Total()

	if n.RawParams.IsSet("disks") {
		for _, rawDiskStr := range n.RawParams.StringSlice("disks") {
			disk := &Disk{}
			if err = disk.Parse(rawDiskStr); err != nil {
				return errors.Wrapf(ErrInvalidDisk, "wrong disk format: %+v, %+v", rawDiskStr, err)
			}
			n.Disks = append(n.Disks, disk)
		}
	}

	if n.RawParams.IsSet("rm-disks") {
		n.RMDisks = strings.Split(n.RawParams.String("rm-disks"), ",")
	}
	return nil
}

// SkipEmpty used for setting node resource capacity in absolute mode
func (n *NodeResourceRequest) SkipEmpty(nodeResource *NodeResource) {
	if n == nil {
		return
	}
	if !n.RawParams.IsSet("volumes") {
		n.Volumes = nodeResource.Volumes
	}
	if !n.RawParams.IsSet("storage") {
		if n.RawParams.IsSet("volumes") {
			n.Storage = nodeResource.Storage - nodeResource.Volumes.Total() + n.Volumes.Total()
		} else {
			n.Storage = nodeResource.Storage
		}
	}
	if !n.RawParams.IsSet("disks") {
		n.Disks = nodeResource.Disks
	}
}
