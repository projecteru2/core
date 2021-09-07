package complexscheduler

import (
	"context"
	"reflect"
	"testing"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func TestVolumeRealloc(t *testing.T) {
	po, err := New(types.Config{})
	assert.Nil(t, err)

	// affinity: no change
	scheduleInfo := func() resourcetypes.ScheduleInfo {
		return resourcetypes.ScheduleInfo{
			NodeMeta: types.NodeMeta{
				Name: "n1",
				InitVolume: types.VolumeMap{
					"/tmp1": 100,
					"/tmp2": 100,
					"/tmp3": 100,
					"/tmp4": 100,
					"/tmp5": 100,
				},
				Volume: types.VolumeMap{
					"/tmp1": 100,
					"/tmp2": 100,
					"/tmp3": 100,
					"/tmp4": 100,
					"/tmp5": 100,
				},
			},
		}
	}
	existing := types.MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data1:rmw:1": {
			"/tmp3": 100,
		},
		"AUTO:/data2:r:2": {
			"/tmp5": 2,
		},
		"AUTO:/data3:rw": {
			"/tmp3": 0,
		},
	})
	req := types.MustToVolumeBindings([]string{"AUTO:/data1:rmw:1", "AUTO:/data2:r:2", "AUTO:/data3:rw"})
	si, volumePlans, total, err := po.ReselectVolumeNodes(context.TODO(), scheduleInfo(), existing, req)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, total)
	assert.EqualValues(t, 1, si.Capacity)
	assert.True(t, reflect.DeepEqual(volumePlans[scheduleInfo().Name][0], existing))

	// affinity: complex no change
	existing = types.MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data1:rmw:1": {
			"/tmp3": 50,
		},
		"AUTO:/data2:r:2": {
			"/tmp5": 2,
		},
		"AUTO:/data3:rw": {
			"/tmp3": 0,
		},
		"AUTO:/data4:rmw:1": {
			"/tmp3": 50,
		},
		"AUTO:/data5:w:20": {
			"/tmp5": 20,
		},
	})
	req = types.MustToVolumeBindings([]string{
		"AUTO:/data1:rmw:1",
		"AUTO:/data2:r:2",
		"AUTO:/data3:rw",
		"AUTO:/data4:rmw:1",
		"AUTO:/data5:w:20",
	})
	si, volumePlans, total, err = po.ReselectVolumeNodes(context.TODO(), scheduleInfo(), existing, req)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, total)
	assert.EqualValues(t, 1, si.Capacity)
	assert.True(t, reflect.DeepEqual(volumePlans[scheduleInfo().Name][0], existing))

	// affinity: increase
	existing = types.MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data1:rmw:1": {
			"/tmp3": 100,
		},
		"AUTO:/data2:r:2": {
			"/tmp5": 2,
		},
	})
	req = types.MustToVolumeBindings([]string{"AUTO:/data1:rmw:2", "AUTO:/data2:r:30"})
	si, volumePlans, total, err = po.ReselectVolumeNodes(context.TODO(), scheduleInfo(), existing, req)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, total)
	assert.EqualValues(t, 1, si.Capacity)
	assert.True(t, reflect.DeepEqual(volumePlans[scheduleInfo().Name][0], types.MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data1:rmw:2": {
			"/tmp3": 100,
		},
		"AUTO:/data2:r:30": {
			"/tmp5": 30,
		},
	})))

	// affinity: decrease
	existing = types.MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data1:rmw:2": {
			"/tmp3": 50,
		},
		"AUTO:/data2:r:54": {
			"/tmp5": 54,
		},
		"AUTO:/data3:rmw:2": {
			"/tmp3": 50,
		},
		"AUTO:/data4:r:34": {
			"/tmp5": 34,
		},
	})
	req = types.MustToVolumeBindings([]string{
		"AUTO:/data1:rmw:1",
		"AUTO:/data2:r:30",
		"AUTO:/data3:rmw:3",
		"AUTO:/data4:r:31",
	})
	si, volumePlans, total, err = po.ReselectVolumeNodes(context.TODO(), scheduleInfo(), existing, req)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, total)
	assert.EqualValues(t, 1, si.Capacity)
	assert.True(t, reflect.DeepEqual(volumePlans[scheduleInfo().Name][0], types.MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data1:rmw:1": {
			"/tmp3": 25,
		},
		"AUTO:/data2:r:30": {
			"/tmp5": 30,
		},
		"AUTO:/data3:rmw:3": {
			"/tmp3": 75,
		},
		"AUTO:/data4:r:31": {
			"/tmp5": 31,
		},
	})))

	// add some new volumes
	existing = types.MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data2:r": {
			"/tmp5": 0,
		},
	})
	req = types.MustToVolumeBindings([]string{
		"AUTO:/data1:rmw:1",
		"AUTO:/data2:r",
		"AUTO:/data3:rmw:3",
	})
	si, volumePlans, total, err = po.ReselectVolumeNodes(context.TODO(), scheduleInfo(), existing, req)
	assert.Nil(t, err)
	assert.EqualValues(t, 5, total)
	assert.EqualValues(t, 5, si.Capacity)
	assert.True(t, reflect.DeepEqual(volumePlans[scheduleInfo().Name][0][types.MustToVolumeBinding("AUTO:/data2:r")], types.VolumeMap{"/tmp5": 0}))

	// del some volumes
	existing = types.MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data1:rmw:2": {
			"/tmp3": 50,
		},
		"AUTO:/data2:r:54": {
			"/tmp5": 54,
		},
		"AUTO:/data3:rmw:2": {
			"/tmp3": 50,
		},
		"AUTO:/data4:r:34": {
			"/tmp5": 34,
		},
	})
	req = types.MustToVolumeBindings([]string{
		"AUTO:/data1:rmw:1",
		"AUTO:/data2:r:30",
	})
	si, volumePlans, total, err = po.ReselectVolumeNodes(context.TODO(), scheduleInfo(), existing, req)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, total)
	assert.EqualValues(t, 1, si.Capacity)
	assert.True(t, reflect.DeepEqual(volumePlans[scheduleInfo().Name][0], types.MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data1:rmw:1": {
			"/tmp3": 100,
		},
		"AUTO:/data2:r:30": {
			"/tmp5": 30,
		},
	})))

	// expand mono error
	existing = types.MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data1:rmw:2": {
			"/tmp3": 100,
		},
	})
	req = types.MustToVolumeBindings([]string{
		"AUTO:/data1:rmw:2",
		"AUTO:/data2:rmw:99",
	})
	si, volumePlans, total, err = po.ReselectVolumeNodes(context.TODO(), scheduleInfo(), existing, req)
	assert.Error(t, err, "no space to expand mono volumes")

	// expand norm error
	existing = types.MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data1:rw:2": {
			"/tmp3": 2,
		},
		"AUTO:/data2:rw:2": {
			"/tmp3": 2,
		},
	})
	req = types.MustToVolumeBindings([]string{
		"AUTO:/data1:rw:3",
		"AUTO:/data2:rw:98",
	})
	si, volumePlans, total, err = po.ReselectVolumeNodes(context.TODO(), scheduleInfo(), existing, req)
	assert.Error(t, err, "no space to expand")

	// reschedule err
	req = types.MustToVolumeBindings([]string{
		"AUTO:/data1:rw:200",
	})
	si, volumePlans, total, err = po.ReselectVolumeNodes(context.TODO(), scheduleInfo(), nil, req)
	assert.Error(t, err, "failed to reschedule")
}
