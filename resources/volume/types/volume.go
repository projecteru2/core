package types

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/projecteru2/core/utils"
)

const auto = "AUTO"

// VolumeBinding src:dst:flags:size
type VolumeBinding struct {
	Source      string
	Destination string
	Flags       string
	SizeInBytes int64
}

// NewVolumeBinding returns pointer of VolumeBinding
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
		if size, err = utils.ParseRAMInHuman(parts[3]); err != nil {
			return nil, errors.WithStack(err)
		}
	default:
		return nil, errors.WithStack(fmt.Errorf("invalid volume: %v", volume))
	}

	flagParts := strings.Split(flags, "")
	sort.Strings(flagParts)

	vb := &VolumeBinding{
		Source:      src,
		Destination: dst,
		Flags:       strings.Join(flagParts, ""),
		SizeInBytes: size,
	}
	return vb, vb.Validate()
}

// Validate return error if invalid
func (vb VolumeBinding) Validate() error {
	if vb.Destination == "" {
		return errors.WithStack(errors.Errorf("invalid volume, dest must be provided: %v", vb))
	}
	if vb.RequireScheduleMonopoly() && vb.RequireScheduleUnlimitedQuota() {
		return errors.WithStack(errors.Errorf("invalid volume, monopoly volume must not be limited: %v", vb))
	}
	return nil
}

// RequireSchedule returns true if volume binding requires schedule
func (vb VolumeBinding) RequireSchedule() bool {
	return strings.HasSuffix(vb.Source, auto)
}

// RequireScheduleUnlimitedQuota .
func (vb VolumeBinding) RequireScheduleUnlimitedQuota() bool {
	return vb.RequireSchedule() && vb.SizeInBytes == 0
}

// RequireScheduleMonopoly returns true if volume binding requires monopoly schedule
func (vb VolumeBinding) RequireScheduleMonopoly() bool {
	return vb.RequireSchedule() && strings.Contains(vb.Flags, "m")
}

// ToString returns volume string
func (vb VolumeBinding) ToString(normalize bool) (volume string) {
	flags := vb.Flags
	if normalize {
		flags = strings.ReplaceAll(flags, "m", "")
	}

	if strings.Contains(flags, "o") {
		flags = strings.ReplaceAll(flags, "o", "")
		flags = strings.ReplaceAll(flags, "r", "ro")
		flags = strings.ReplaceAll(flags, "w", "wo")
	}

	switch {
	case vb.Flags == "" && vb.SizeInBytes == 0:
		volume = fmt.Sprintf("%s:%s", vb.Source, vb.Destination)
	default:
		volume = fmt.Sprintf("%s:%s:%s:%d", vb.Source, vb.Destination, flags, vb.SizeInBytes)
	}
	return volume
}

// GetMapKey .
func (vb VolumeBinding) GetMapKey() [3]string {
	return [3]string{vb.Source, vb.Destination, vb.Flags}
}

// VolumeBindings is a collection of VolumeBinding
type VolumeBindings []*VolumeBinding

// NewVolumeBindings return VolumeBindings of reference type
func NewVolumeBindings(volumes []string) (volumeBindings VolumeBindings, err error) {
	for _, vb := range volumes {
		volumeBinding, err := NewVolumeBinding(vb)
		if err != nil {
			return nil, err
		}
		volumeBindings = append(volumeBindings, volumeBinding)
	}
	return
}

// UnmarshalJSON is used for encoding/json.Unmarshal
func (vbs *VolumeBindings) UnmarshalJSON(b []byte) (err error) {
	volumes := []string{}
	if err = json.Unmarshal(b, &volumes); err != nil {
		return errors.WithStack(err)
	}
	*vbs, err = NewVolumeBindings(volumes)
	return
}

// MarshalJSON is used for encoding/json.Marshal
func (vbs VolumeBindings) MarshalJSON() ([]byte, error) {
	volumes := []string{}
	for _, vb := range vbs {
		volumes = append(volumes, vb.ToString(false))
	}
	bs, err := json.Marshal(volumes)
	return bs, errors.WithStack(err)
}

// TotalSize .
func (vbs VolumeBindings) TotalSize() (total int64) {
	for _, vb := range vbs {
		total += vb.SizeInBytes
	}
	return
}

// ApplyPlan creates new VolumeBindings according to volume plan
func (vbs VolumeBindings) ApplyPlan(plan VolumePlan) (res VolumeBindings) {
	for _, vb := range vbs {
		newVb := &VolumeBinding{vb.Source, vb.Destination, vb.Flags, vb.SizeInBytes}
		if vmap, _ := plan.GetVolumeMap(vb); vmap != nil {
			newVb.Source = vmap.GetDevice()
		}
		res = append(res, newVb)
	}
	return
}

// MergeVolumeBindings combines two VolumeBindings
func MergeVolumeBindings(vbs1 VolumeBindings, vbs2 ...VolumeBindings) (vbs VolumeBindings) {
	sizeMap := map[[3]string]int64{} // {["AUTO", "/data", "rw"]: 100}
	for _, vbs := range append(vbs2, vbs1) {
		for _, vb := range vbs {
			sizeMap[vb.GetMapKey()] += vb.SizeInBytes
		}
	}

	for key, size := range sizeMap {
		if size < 0 {
			continue
		}
		vbs = append(vbs, &VolumeBinding{
			Source:      key[0],
			Destination: key[1],
			Flags:       key[2],
			SizeInBytes: size,
		})
	}
	return
}

// VolumeMap .
type VolumeMap map[string]int64

// DeepCopy .
func (v VolumeMap) DeepCopy() VolumeMap {
	res := VolumeMap{}
	for key, value := range v {
		res[key] = value
	}
	return res
}

// Add .
func (v VolumeMap) Add(v1 VolumeMap) {
	for key, value := range v1 {
		v[key] += value
	}
}

// Sub .
func (v VolumeMap) Sub(v1 VolumeMap) {
	for key, value := range v1 {
		v[key] -= value
	}
}

// GetDevice returns the first device
func (v VolumeMap) GetDevice() string {
	for key := range v {
		return key
	}
	return ""
}

// GetSize returns the first size
func (v VolumeMap) GetSize() int64 {
	for _, size := range v {
		return size
	}
	return 0
}

// Total .
func (v VolumeMap) Total() int64 {
	res := int64(0)
	for _, size := range v {
		res += size
	}
	return res
}

// VolumePlan is map from volume string to volumeMap: {"AUTO:/data:rw:100": VolumeMap{"/sda1": 100}}
type VolumePlan map[*VolumeBinding]VolumeMap

// UnmarshalJSON .
func (p *VolumePlan) UnmarshalJSON(b []byte) (err error) {
	if *p == nil {
		*p = VolumePlan{}
	}
	plan := map[string]VolumeMap{}
	if err = json.Unmarshal(b, &plan); err != nil {
		return errors.WithStack(err)
	}
	for volume, vmap := range plan {
		vb, err := NewVolumeBinding(volume)
		if err != nil {
			return errors.WithStack(err)
		}
		(*p)[vb] = vmap
	}
	return
}

// MarshalJSON .
func (p VolumePlan) MarshalJSON() ([]byte, error) {
	plan := map[string]VolumeMap{}
	for vb, vmap := range p {
		plan[vb.ToString(false)] = vmap
	}
	bs, err := json.Marshal(plan)
	return bs, errors.WithStack(err)
}

// Merge .
func (p VolumePlan) Merge(p2 VolumePlan) {
	for vb, vm := range p2 {
		if oldVM, oldVB := p.GetVolumeMap(vb); oldVB != nil {
			delete(p, oldVB)
			vm[vm.GetDevice()] += oldVM.GetSize()
			vm = VolumeMap{vm.GetDevice(): vm.GetSize() + oldVM.GetSize()}
			vb = &VolumeBinding{
				Source:      vb.Source,
				Destination: vb.Destination,
				Flags:       vb.Flags,
				SizeInBytes: vb.SizeInBytes + oldVB.SizeInBytes,
			}
		}
		p[vb] = vm
	}
}

// GetVolumeMap looks up VolumeMap according to volume destination directory
func (p VolumePlan) GetVolumeMap(vb *VolumeBinding) (volMap VolumeMap, volume *VolumeBinding) {
	for volume, volMap := range p {
		if vb.Destination == volume.Destination {
			return volMap, volume
		}
	}
	return
}
