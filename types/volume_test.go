package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func NormalVolumeBindingTestcases(t *testing.T) (testcases []*VolumeBinding) {
	vb, err := NewVolumeBinding("/src:/dst:rwm:1000")
	assert.Nil(t, err)
	assert.Equal(t, vb, &VolumeBinding{"/src", "/dst", "mrw", int64(1000)})
	assert.False(t, vb.RequireSchedule())
	assert.False(t, vb.RequireScheduleMonopoly())
	testcases = append(testcases, vb)

	vb, err = NewVolumeBinding("/src:/dst:rwm")
	assert.Nil(t, err)
	assert.Equal(t, vb, &VolumeBinding{"/src", "/dst", "mrw", int64(0)})
	assert.False(t, vb.RequireSchedule())
	assert.False(t, vb.RequireScheduleMonopoly())
	testcases = append(testcases, vb)

	vb, err = NewVolumeBinding("/src:/dst")
	assert.Nil(t, err)
	assert.Equal(t, vb, &VolumeBinding{"/src", "/dst", "", int64(0)})
	assert.False(t, vb.RequireSchedule())
	assert.False(t, vb.RequireScheduleMonopoly())
	testcases = append(testcases, vb)

	return
}

func AutoVolumeBindingTestcases(t *testing.T) (testcases []*VolumeBinding) {
	vb, err := NewVolumeBinding("AUTO:/data:rw:1")
	assert.Nil(t, err)
	assert.True(t, vb.RequireSchedule())
	assert.False(t, vb.RequireScheduleMonopoly())
	testcases = append(testcases, vb)

	vb, err = NewVolumeBinding("AUTO:/dir:rwm:1")
	assert.Nil(t, err)
	assert.True(t, vb.RequireSchedule())
	assert.True(t, vb.RequireScheduleMonopoly())
	testcases = append(testcases, vb)

	return
}

func TestNewVolumeBinding(t *testing.T) {
	NormalVolumeBindingTestcases(t)
	AutoVolumeBindingTestcases(t)

	_, err := NewVolumeBinding("/src:/dst:rw:1G")
	assert.Error(t, err, "invalid syntax")

	_, err = NewVolumeBinding("/src:/dst:rwm:1:asdf")
	assert.Error(t, err, "invalid volume")

	_, err = NewVolumeBinding("/src:/data:rw:-1")
	assert.Nil(t, err)

	_, err = NewVolumeBinding("AUTO:/data:rw")
	assert.Nil(t, err)

	_, err = NewVolumeBinding("AUTO::rw:1")
	assert.Error(t, err, "dest must be provided")

	_, err = NewVolumeBinding("AUTO:/data:rmo:0")
	assert.Error(t, err, "monopoly volume must not be limited")
}

func TestVolumeBindingToString(t *testing.T) {
	cases := NormalVolumeBindingTestcases(t)
	assert.Equal(t, cases[0].ToString(false), "/src:/dst:mrw:1000")
	assert.Equal(t, cases[1].ToString(false), "/src:/dst:mrw:0")
	assert.Equal(t, cases[2].ToString(false), "/src:/dst")
	assert.Equal(t, cases[1].ToString(true), "/src:/dst:rw:0")
}

func TestVolumeBindings(t *testing.T) {
	_, err := NewVolumeBindings([]string{"/1::rw:0"})
	assert.Error(t, err, "dest must be provided")
	vbs, _ := NewVolumeBindings([]string{"/1:/dst:rw:1000", "/0:/dst:rom"})
	assert.Equal(t, vbs.ToStringSlice(false, false), []string{"/1:/dst:rw:1000", "/0:/dst:mro:0"})
	assert.Equal(t, vbs.ToStringSlice(true, false), []string{"/0:/dst:mro:0", "/1:/dst:rw:1000"})
	assert.Equal(t, vbs.TotalSize(), int64(1000))

	vbs1, _ := NewVolumeBindings([]string{"AUTO:/data0:rw:1", "AUTO:/data1:rw:2", "/mnt1:/data2:rw", "/mnt2:/data3:ro"})
	vbs2, _ := NewVolumeBindings([]string{"AUTO:/data7:rw:3", "AUTO:/data1:rw:3", "/mnt3:/data8", "AUTO:/data0:rw:-20"})
	vbs = MergeVolumeBindings(vbs1, vbs2)
	softVolumes, hardVolumes := vbs.Divide()
	assert.Equal(t, softVolumes.ToStringSlice(true, false), []string{"AUTO:/data1:rw:5", "AUTO:/data7:rw:3"})
	assert.Equal(t, hardVolumes.ToStringSlice(true, false), []string{"/mnt1:/data2:rw:0", "/mnt2:/data3:ro:0", "/mnt3:/data8"})

	assert.True(t, vbs1.IsEqual(vbs1))
	assert.False(t, vbs1.IsEqual(vbs2))

	vp := VolumePlan{
		MustToVolumeBinding("AUTO:/data0:rw:1"): VolumeMap{"/mnt0": 1},
		MustToVolumeBinding("AUTO:/data1:rm:2"): VolumeMap{"/mnt1": 2},
		MustToVolumeBinding("AUTO:/data7:rw:3"): VolumeMap{"/mnt2": 3},
	}
	vbs = vbs1.ApplyPlan(vp)
	assert.True(t, MustToVolumeBindings([]string{"/mnt0:/data0:rw:1", "/mnt1:/data1:rw:2", "/mnt1:/data2:rw", "/mnt2:/data3:ro"}).IsEqual(vbs))
}

func TestVolumeBindingsJSONEncoding(t *testing.T) {
	vbs := MustToVolumeBindings([]string{"AUTO:/data0:rw:1", "AUTO:/data1:rw:2", "/mnt1:/data2:rw", "/mnt2:/data3:ro"})
	data := []byte(`["AUTO:/data0:rw:1","AUTO:/data1:rw:2","/mnt1:/data2:rw:0","/mnt2:/data3:ro:0"]`)
	b, err := json.Marshal(vbs)
	assert.Nil(t, err)
	assert.Equal(t, b, data)

	vbs1 := VolumeBindings{}
	err = json.Unmarshal(data, &vbs1)
	assert.Nil(t, err)
	assert.Equal(t, vbs1, vbs)
}
