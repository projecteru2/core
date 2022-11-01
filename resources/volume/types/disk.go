package types

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/projecteru2/core/utils"
)

// Disk .
type Disk struct {
	Device    string   `json:"device"`
	Mounts    []string `json:"mounts"`
	ReadIOPS  int64    `json:"read_IOPS"`
	WriteIOPS int64    `json:"write_IOPS"`
	ReadBPS   int64    `json:"read_bps"`
	WriteBPS  int64    `json:"write_bps"`
}

// String .
func (d *Disk) String() string {
	return fmt.Sprintf("%v:%v:%v:%v:%v:%v", d.Device, strings.Join(d.Mounts, ","), d.ReadIOPS, d.WriteIOPS, d.ReadBPS, d.WriteBPS)
}

// ParseFromString .
func (d *Disk) ParseFromString(s string) (err error) {
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return ErrInvalidStorage
	}
	d.Device = parts[0]
	d.Mounts = strings.Split(parts[1], ",")
	if d.ReadIOPS, err = strconv.ParseInt(parts[2], 10, 64); err != nil {
		return err
	}
	if d.WriteIOPS, err = strconv.ParseInt(parts[3], 10, 64); err != nil {
		return err
	}
	if d.ReadBPS, err = utils.ParseRAMInHuman(parts[4]); err != nil {
		return err
	}
	if d.WriteBPS, err = utils.ParseRAMInHuman(parts[5]); err != nil {
		return err
	}
	return nil
}

// DeepCopy .
func (d *Disk) DeepCopy() *Disk {
	return &Disk{
		Device:    d.Device,
		Mounts:    d.Mounts,
		ReadIOPS:  d.ReadIOPS,
		WriteIOPS: d.WriteIOPS,
		ReadBPS:   d.ReadBPS,
		WriteBPS:  d.WriteBPS,
	}
}

// Disks .
type Disks []*Disk

// DeepCopy .
func (d Disks) DeepCopy() Disks {
	disks := make(Disks, len(d))
	for i, disk := range d {
		disks[i] = &Disk{
			Device:    disk.Device,
			Mounts:    disk.Mounts,
			ReadIOPS:  disk.ReadIOPS,
			WriteIOPS: disk.WriteIOPS,
			ReadBPS:   disk.ReadBPS,
			WriteBPS:  disk.WriteBPS,
		}
	}
	return disks
}

// GetDiskByDevice .
func (d Disks) GetDiskByDevice(device string) *Disk {
	for _, disk := range d {
		if disk.Device == device {
			return disk
		}
	}
	return nil
}

// GetDiskByPath .
func (d Disks) GetDiskByPath(path string) *Disk {
	mountToDiskMap := map[string]*Disk{}
	mounts := []string{}
	for _, disk := range d {
		for _, mount := range disk.Mounts {
			mount = addSlash(mount)
			mountToDiskMap[mount] = disk
			mounts = append(mounts, mount)
		}
	}

	// sort the mounts by the number of delimiters (descending)
	sort.Slice(mounts, func(i, j int) bool {
		return getDelimiterCount(mounts[i], '/') > getDelimiterCount(mounts[j], '/')
	})

	for _, mount := range mounts {
		if hasPrefix(path, mount) {
			return mountToDiskMap[mount]
		}
	}
	return nil
}

// Add .
func (d *Disks) Add(d1 Disks) {
	toAppend := []*Disk{}
	for _, disk1 := range d1 {
		disk := d.GetDiskByDevice(disk1.Device)
		if disk != nil {
			disk.ReadIOPS += disk1.ReadIOPS
			disk.WriteIOPS += disk1.WriteIOPS
			disk.ReadBPS += disk1.ReadBPS
			disk.WriteBPS += disk1.WriteBPS
			if len(disk1.Mounts) > 0 {
				disk.Mounts = disk1.Mounts
			}
		} else {
			toAppend = append(toAppend, disk1.DeepCopy())
		}
	}

	for _, disk := range toAppend {
		*d = append(*d, disk.DeepCopy())
	}
}

// Sub .
func (d *Disks) Sub(d1 Disks) {
	toAppend := []*Disk{}
	for _, disk1 := range d1 {
		disk := d.GetDiskByDevice(disk1.Device)
		if disk != nil {
			disk.ReadIOPS -= disk1.ReadIOPS
			disk.WriteIOPS -= disk1.WriteIOPS
			disk.ReadBPS -= disk1.ReadBPS
			disk.WriteBPS -= disk1.WriteBPS
			if len(disk1.Mounts) > 0 {
				disk.Mounts = disk1.Mounts
			}
		} else {
			toAppend = append(toAppend, disk1.DeepCopy())
		}
	}

	for _, disk := range toAppend {
		*d = append(*d, &Disk{
			Device:    disk.Device,
			Mounts:    disk.Mounts,
			ReadIOPS:  -disk.ReadIOPS,
			WriteIOPS: -disk.WriteIOPS,
			ReadBPS:   -disk.ReadBPS,
			WriteBPS:  -disk.WriteBPS,
		})
	}
}

// RemoveMounts remove mounts so Add / Sub won't affect the origin mounts
func (d *Disks) RemoveMounts() Disks {
	disks := d.DeepCopy()
	for _, disk := range disks {
		disk.Mounts = nil
	}
	return disks
}

func getDelimiterCount(str string, delimiter int32) int {
	count := 0
	for _, c := range str {
		if c == delimiter {
			count++
		}
	}
	return count
}

func hasPrefix(path string, mount string) bool {
	// /data -> /data/xxx
	// /data -> /data
	// / -> /asdf
	mount = addSlash(mount)
	path = addSlash(path)
	return strings.HasPrefix(path, mount)
}

func addSlash(dir string) string {
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	return dir
}
