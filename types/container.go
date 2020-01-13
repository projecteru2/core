package types

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	engine "github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
)

type VolumeMode string

const ()

// StatusMeta indicate contaienr runtime
type StatusMeta struct {
	ID string `json:"id"`

	Networks  map[string]string `json:"networks,omitempty"`
	Running   bool              `json:"running,omitempty"`
	Healthy   bool              `json:"healthy,omitempty"`
	Extension []byte            `json:"extension,omitempty"`
}

// LabelMeta bind meta info store in labels
type LabelMeta struct {
	Publish     []string
	HealthCheck *HealthCheck
}

// VolumeBinding: src:dst:flags:size
type VolumeBinding struct {
	Source      string
	Destination string
	Flags       string
	SizeInBytes int64
}

func NewVolumeBinding(volume string) (_ *VolumeBinding, err error) {
	var src, dst, flags string
	var size int64

	parts := strings.Split(volume, ":")
	switch len(parts) {
	case 2:
		src, dst = parts[0], parts[1]
	case 3:
		src, dst, flags = parts[0], parts[1], parts[2]
	case 4:
		src, dst, flags = parts[0], parts[1], parts[2]
		if size, err = strconv.ParseInt(parts[3], 10, 64); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid volume: %v", volume)
	}

	return &VolumeBinding{
		Source:      src,
		Destination: dst,
		Flags:       flags,
		SizeInBytes: size,
	}, nil
}

func (vb VolumeBinding) NeedSchedule() bool {
	return vb.Source == AUTO && vb.Destination != "" && vb.Flags != "" && vb.SizeInBytes != 0
}

func (vb VolumeBinding) ToString() string {
	volume := fmt.Sprintf("%s:%s:%s:%d", vb.Source, vb.Destination, vb.Flags, vb.SizeInBytes)
	return strings.TrimRight(volume, ":")
}

type VolumeBindings []*VolumeBinding

func MakeVolumeBindings(volumes []string) (volumeBindings VolumeBindings, err error) {
	for _, vb := range volumes {
		volumeBinding, err := NewVolumeBinding(vb)
		if err != nil {
			return nil, err
		}
		volumeBindings = append(volumeBindings, volumeBinding)
	}
	return
}

func (vbs VolumeBindings) ToStringSlice(sorted bool) (volumes []string) {
	if sorted {
		sort.Slice(vbs, func(i, j int) bool { return vbs[i].ToString() < vbs[j].ToString() })
	}
	for _, vb := range vbs {
		volumes = append(volumes, vb.ToString())
	}
	return
}

func (vbs *VolumeBindings) UnmarshalJSON(b []byte) (err error) {
	volumes := []string{}
	if err = json.Unmarshal(b, &volumes); err != nil {
		return err
	}
	*vbs, err = MakeVolumeBindings(volumes)
	return
}

func (vbs *VolumeBindings) MarshalJSON() ([]byte, error) {
	volumes := []string{}
	for _, vb := range *vbs {
		volumes = append(volumes, vb.ToString())
	}
	return json.Marshal(volumes)
}

func (vbs VolumeBindings) AdditionalStorage() (storage int64) {
	for _, vb := range vbs {
		storage += vb.SizeInBytes
	}
	return
}

func (vbs VolumeBindings) ApplyPlan(plan VolumePlan) (res VolumeBindings) {
	for _, vb := range vbs {
		newVb := &VolumeBinding{vb.Source, vb.Destination, vb.Flags, vb.SizeInBytes}
		if vmap := plan.GetVolumeMap(vb); vmap != nil {
			newVb.Source = vmap.GetResourceID()
		}
		res = append(res, newVb)
	}
	return vbs
}

func (vbs VolumeBindings) Merge(vbs2 VolumeBindings) (softVolumes VolumeBindings, hardVolumes VolumeBindings, err error) {
	sizeMap := map[[3]string]int64{} // {["AUTO", "/data", "rw"]: 100}
	for _, vb := range append(vbs, vbs2...) {
		if !vb.NeedSchedule() {
			hardVolumes = append(hardVolumes, vb)
			continue
		}
		key := [3]string{vb.Source, vb.Destination, vb.Flags}
		if _, ok := sizeMap[key]; ok {
			sizeMap[key] += vb.SizeInBytes
		} else {
			sizeMap[key] = vb.SizeInBytes
		}
	}

	for key, size := range sizeMap {
		if size < 0 {
			continue
		}
		softVolumes = append(softVolumes, &VolumeBinding{key[0], key[1], key[2], size})
	}
	return
}

func (vbs VolumeBindings) IsEqual(vbs2 VolumeBindings) bool {
	if len(vbs) != len(vbs2) {
		return false
	}

	for idx, vb := range vbs {
		if vb != vbs2[idx] {
			return false
		}
	}
	return true
}

// Container store container info
// only relationship with pod and node is stored
// if you wanna get realtime information, use Inspect method
type Container struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Podname    string            `json:"podname"`
	Nodename   string            `json:"nodename"`
	CPU        CPUMap            `json:"cpu"`
	Quota      float64           `json:"quota"`
	Memory     int64             `json:"memory"`
	Storage    int64             `json:"storage"`
	Hook       *Hook             `json:"hook"`
	Privileged bool              `json:"privileged"`
	SoftLimit  bool              `json:"softlimit"`
	User       string            `json:"user"`
	Env        []string          `json:"env"`
	Image      string            `json:"image"`
	Volumes    VolumeBindings    `json:"volumes"`
	VolumePlan VolumePlan        `json:"volume_plan"`
	Labels     map[string]string `json:"labels"`
	StatusMeta *StatusMeta       `json:"-"`
	Engine     engine.API        `json:"-"`
}

// Inspect a container
func (c *Container) Inspect(ctx context.Context) (*enginetypes.VirtualizationInfo, error) {
	if c.Engine == nil {
		return nil, ErrNilEngine
	}
	return c.Engine.VirtualizationInspect(ctx, c.ID)
}

// Start a container
func (c *Container) Start(ctx context.Context) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	return c.Engine.VirtualizationStart(ctx, c.ID)
}

// Stop a container
func (c *Container) Stop(ctx context.Context) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	return c.Engine.VirtualizationStop(ctx, c.ID)
}

// Remove a container
func (c *Container) Remove(ctx context.Context, force bool) error {
	if c.Engine == nil {
		return ErrNilEngine
	}
	return c.Engine.VirtualizationRemove(ctx, c.ID, true, force)
}

// ContainerStatus store deploy status
type ContainerStatus struct {
	ID        string
	Container *Container
	Error     error
	Delete    bool
}
