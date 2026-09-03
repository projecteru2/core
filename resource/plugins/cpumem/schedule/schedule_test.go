package schedule

import (
	"math/rand/v2"
	"reflect"
	"strconv"
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resource/plugins/cpumem/types"
)

func TestGetFullCPUPlans(t *testing.T) {
	h := newHost(types.CPUMap{
		"0": 400,
		"1": 200,
		"2": 400,
	}, 100, -1)
	cpuPlans := h.getFullCPUPlans(h.fullCores, 2)
	assert.Equal(t, 5, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []types.CPUMap{
		{"0": 100, "1": 100},
		{"0": 100, "2": 100},
		{"0": 100, "2": 100},
		{"0": 100, "2": 100},
		{"1": 100, "2": 100},
	})

	h = newHost(types.CPUMap{
		"0": 200,
		"1": 200,
		"2": 200,
	}, 100, -1)
	cpuPlans = h.getFullCPUPlans(h.fullCores, 2)
	assert.EqualValues(t, 3, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []types.CPUMap{
		{"0": 100, "1": 100},
		{"0": 100, "2": 100},
		{"1": 100, "2": 100},
	})
}

func TestGetCPUPlansWithAffinity(t *testing.T) {
	cases := []struct {
		name         string
		cpuMap       types.CPUMap
		originCPUMap types.CPUMap
		cpuRequest   float64
		want         []*types.CPUPlan
	}{
		{
			name:         "single core exact fit",
			cpuMap:       types.CPUMap{"0": 0, "1": 30, "2": 0},
			originCPUMap: types.CPUMap{"0": 100, "1": 30, "2": 40},
			cpuRequest:   1,
			want:         []*types.CPUPlan{{CPUMap: types.CPUMap{"0": 100}}},
		},
		{
			name:         "fractional request spills into second core",
			cpuMap:       types.CPUMap{"0": 0, "1": 30, "2": 0},
			originCPUMap: types.CPUMap{"0": 100, "1": 30, "2": 40},
			cpuRequest:   1.2,
			want:         []*types.CPUPlan{{CPUMap: types.CPUMap{"0": 100, "1": 20}}},
		},
		{
			name:         "two full cores",
			cpuMap:       types.CPUMap{"0": 0, "1": 80, "2": 0, "3": 0},
			originCPUMap: types.CPUMap{"0": 100, "1": 20, "2": 40, "3": 10},
			cpuRequest:   2,
			want:         []*types.CPUPlan{{CPUMap: types.CPUMap{"0": 100, "1": 100}}},
		},
		{
			name:         "insufficient affinity capacity",
			cpuMap:       types.CPUMap{"0": 0, "1": 69, "2": 10},
			originCPUMap: types.CPUMap{"0": 100, "1": 30, "2": 40},
			cpuRequest:   2,
			want:         nil,
		},
		{
			name:         "just enough affinity capacity",
			cpuMap:       types.CPUMap{"0": 0, "1": 70, "2": 10},
			originCPUMap: types.CPUMap{"0": 100, "1": 30, "2": 40},
			cpuRequest:   2,
			want:         []*types.CPUPlan{{CPUMap: types.CPUMap{"0": 100, "1": 100}}},
		},
		{
			name:         "multiple candidate second cores",
			cpuMap:       types.CPUMap{"0": 100, "1": 60, "2": 0, "3": 100, "4": 100},
			originCPUMap: types.CPUMap{"0": 100, "1": 30, "2": 40},
			cpuRequest:   2,
			want: []*types.CPUPlan{
				{CPUMap: types.CPUMap{"0": 100, "3": 100}},
				{CPUMap: types.CPUMap{"0": 100, "4": 100}},
			},
		},
		{
			name:         "no full core available",
			cpuMap:       types.CPUMap{"0": 0, "1": 60, "2": 0},
			originCPUMap: types.CPUMap{"0": 100, "1": 30, "2": 40},
			cpuRequest:   2,
			want:         nil,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			resourceInfo := &types.NodeResourceInfo{
				Capacity: &types.NodeResource{CPUMap: tt.cpuMap, CPU: float64(len(tt.cpuMap))},
				Usage:    &types.NodeResource{},
			}
			resourceInfo.Capacity.CPUMap.Add(tt.originCPUMap)
			cpuPlans := GetCPUPlans(resourceInfo, tt.originCPUMap, 100, -1, &types.WorkloadResourceRequest{CPUBind: true, CPURequest: tt.cpuRequest})
			assert.Equal(t, len(tt.want), len(cpuPlans))
			assert.ElementsMatch(t, cpuPlans, tt.want)
		})
	}
}

func TestCPUOverSell(t *testing.T) {
	maxShare := -1
	shareBase := 100

	cases := []struct {
		name        string
		cpuMap      types.CPUMap
		cpuRequest  float64
		wantLen     int
		wantAtLeast int
		want        []*types.CPUPlan
		wantPrefix  []*types.CPUPlan
	}{
		{
			name:       "two full cores split three ways",
			cpuMap:     types.CPUMap{"0": 300, "1": 300},
			cpuRequest: 2,
			wantLen:    3,
			want: []*types.CPUPlan{
				{CPUMap: types.CPUMap{"0": 100, "1": 100}},
				{CPUMap: types.CPUMap{"0": 100, "1": 100}},
				{CPUMap: types.CPUMap{"0": 100, "1": 100}},
			},
		},
		{
			name:       "single core fractional oversell",
			cpuMap:     types.CPUMap{"0": 300},
			cpuRequest: 0.5,
			wantLen:    6,
			want: []*types.CPUPlan{
				{CPUMap: types.CPUMap{"0": 50}},
				{CPUMap: types.CPUMap{"0": 50}},
				{CPUMap: types.CPUMap{"0": 50}},
				{CPUMap: types.CPUMap{"0": 50}},
				{CPUMap: types.CPUMap{"0": 50}},
				{CPUMap: types.CPUMap{"0": 50}},
			},
		},
		{
			name:       "three cores whole request",
			cpuMap:     types.CPUMap{"0": 100, "1": 200, "2": 300},
			cpuRequest: 1,
			wantLen:    6,
			wantPrefix: []*types.CPUPlan{
				{CPUMap: types.CPUMap{"0": 100}},
				{CPUMap: types.CPUMap{"1": 100}},
			},
		},
		{
			name:        "seven fragmented cores lower bound only",
			cpuMap:      types.CPUMap{"0": 50, "1": 100, "2": 300, "3": 70, "4": 200, "5": 30, "6": 230},
			cpuRequest:  1.7,
			wantAtLeast: 2,
		},
		{
			name:       "three cores fragment plus full core",
			cpuMap:     types.CPUMap{"0": 70, "1": 100, "2": 400},
			cpuRequest: 1.3,
			wantLen:    4,
			want: []*types.CPUPlan{
				{CPUMap: types.CPUMap{"0": 30, "2": 100}},
				{CPUMap: types.CPUMap{"0": 30, "2": 100}},
				{CPUMap: types.CPUMap{"1": 30, "2": 100}},
				{CPUMap: types.CPUMap{"1": 30, "2": 100}},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			resourceInfo := &types.NodeResourceInfo{Capacity: &types.NodeResource{
				CPU:    float64(len(tt.cpuMap)),
				CPUMap: tt.cpuMap,
				Memory: 12 * units.GiB,
			}}
			assert.Nil(t, resourceInfo.Validate())

			cpuPlans := GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceRequest{
				CPUBind:    true,
				CPURequest: tt.cpuRequest,
				MemRequest: 1,
			})
			if tt.wantAtLeast > 0 {
				assert.True(t, len(cpuPlans) >= tt.wantAtLeast)
				return
			}
			assert.Equal(t, len(cpuPlans), tt.wantLen)
			if tt.wantPrefix != nil {
				assert.ElementsMatch(t, cpuPlans[:len(tt.wantPrefix)], tt.wantPrefix)
				return
			}
			assert.ElementsMatch(t, cpuPlans, tt.want)
		})
	}
}

func TestCPUOverSellAndStableFragmentCore(t *testing.T) {
	maxShare := -1
	shareBase := 100

	cpuMap := types.CPUMap{"0": 300, "1": 300}
	resourceInfo := &types.NodeResourceInfo{Capacity: &types.NodeResource{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans := GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceRequest{
		CPUBind:    true,
		CPURequest: 1.7,
		MemRequest: 1,
	})
	assert.True(t, len(cpuPlans) > 0)

	cpuMap = types.CPUMap{"0": 230, "1": 200}
	resourceInfo = &types.NodeResourceInfo{Capacity: &types.NodeResource{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceRequest{
		CPUBind:    true,
		CPURequest: 1.7,
		MemRequest: 1,
	})
	assert.True(t, len(cpuPlans) > 0)
	assert.Equal(t, cpuPlans[0].CPUMap, types.CPUMap{"0": 70, "1": 100})
	applyCPUPlans(t, resourceInfo, cpuPlans[:1])

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceRequest{
		CPUBind:    true,
		CPURequest: 1.7,
		MemRequest: 1,
	})
	assert.True(t, len(cpuPlans) > 0)
	assert.Equal(t, cpuPlans[0].CPUMap, types.CPUMap{"0": 70, "1": 100})

	cases := []struct {
		name       string
		cpuMap     types.CPUMap
		cpuRequest float64
		wantUsage  types.CPUMap
	}{
		{
			name:       "four cores two fragments",
			cpuMap:     types.CPUMap{"0": 230, "1": 80, "2": 300, "3": 200},
			cpuRequest: 1.7,
			wantUsage:  types.CPUMap{"0": 70, "1": 70, "2": 0, "3": 200},
		},
		{
			name:       "five cores mixed sizes",
			cpuMap:     types.CPUMap{"0": 70, "1": 50, "2": 100, "3": 100, "4": 100},
			cpuRequest: 1.7,
			wantUsage:  types.CPUMap{"0": 70, "1": 0, "2": 70, "3": 100, "4": 100},
		},
		{
			name:       "three cores fractional request",
			cpuMap:     types.CPUMap{"0": 70, "1": 50, "2": 90},
			cpuRequest: 0.5,
			wantUsage:  types.CPUMap{"0": 50, "1": 50, "2": 0},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			resourceInfo := &types.NodeResourceInfo{Capacity: &types.NodeResource{
				CPU:    float64(len(tt.cpuMap)),
				CPUMap: tt.cpuMap,
				Memory: 12 * units.GiB,
			}}
			assert.Nil(t, resourceInfo.Validate())

			cpuPlans := GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceRequest{
				CPUBind:    true,
				CPURequest: tt.cpuRequest,
				MemRequest: 1,
			})
			assert.True(t, len(cpuPlans) >= 2)
			applyCPUPlans(t, resourceInfo, cpuPlans[:2])
			assert.Equal(t, resourceInfo.Usage.CPUMap, tt.wantUsage)
		})
	}
}

func TestNUMANodes(t *testing.T) {
	maxShare := -1
	shareBase := 100

	resourceInfo := &types.NodeResourceInfo{
		Capacity: &types.NodeResource{
			CPU:        4,
			CPUMap:     types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
			Memory:     4 * units.GiB,
			NUMAMemory: types.NUMAMemory{"0": 2 * units.GiB, "1": 2 * units.GiB},
			NUMA:       types.NUMA{"0": "0", "1": "0", "2": "1", "3": "1"},
		},
		Usage: nil,
	}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans := GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceRequest{
		CPUBind:    true,
		CPURequest: 1.3,
		MemRequest: 1,
	})
	assert.Equal(t, 2, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []*types.CPUPlan{
		{CPUMap: types.CPUMap{"0": 30, "1": 100}, NUMANode: "0"},
		{CPUMap: types.CPUMap{"2": 30, "3": 100}, NUMANode: "1"},
	})

	resourceInfo = &types.NodeResourceInfo{
		Capacity: &types.NodeResource{
			CPU:        4,
			CPUMap:     types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100, "4": 100, "5": 100},
			Memory:     6 * units.GiB,
			NUMAMemory: types.NUMAMemory{"0": 3 * units.GiB, "1": 3 * units.GiB},
			NUMA:       types.NUMA{"0": "0", "1": "0", "2": "0", "3": "1", "4": "1", "5": "1"},
		},
		Usage: nil,
	}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceRequest{
		CPUBind:    true,
		CPURequest: 2,
		MemRequest: 2 * units.GiB,
	})
	assert.Equal(t, 3, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []*types.CPUPlan{
		{CPUMap: types.CPUMap{"1": 100, "2": 100}, NUMANode: "0"},
		{CPUMap: types.CPUMap{"4": 100, "5": 100}, NUMANode: "1"},
		{CPUMap: types.CPUMap{"0": 100, "3": 100}, NUMANode: ""},
	})
}

func TestInsufficientMemory(t *testing.T) {
	maxShare := -1
	shareBase := 100

	resourceInfo := &types.NodeResourceInfo{
		Capacity: &types.NodeResource{
			CPU:    4,
			CPUMap: types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
			Memory: 4 * units.GiB,
		},
		Usage: nil,
	}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans := GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceRequest{
		CPUBind:    true,
		CPURequest: 1.3,
		MemRequest: 3 * units.GiB,
	})
	assert.Equal(t, 1, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []*types.CPUPlan{
		{CPUMap: types.CPUMap{"0": 30, "1": 100}},
	})
}

func TestFragmentCoresAboveMaxShare(t *testing.T) {
	cpuMap := types.CPUMap{"0": 100, "1": 100, "2": 30, "3": 40, "4": 50}
	resourceInfo := &types.NodeResourceInfo{Capacity: &types.NodeResource{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans := GetCPUPlans(resourceInfo, nil, 100, 2, &types.WorkloadResourceRequest{
		CPUBind:    true,
		CPURequest: 0.5,
		MemRequest: 1,
	})
	assert.Equal(t, 1, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []*types.CPUPlan{{CPUMap: types.CPUMap{"4": 50}}})
}

func TestGetCPUPlansMatchTheLinearSplitScan(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 11))
	for range 300 {
		cores := 1 + rng.IntN(40)
		cpuMap := types.CPUMap{}
		for i := range cores {
			cpuMap[strconv.Itoa(i)] = rng.IntN(4) * 25
		}
		shareBase, maxFragment := 100, -1+rng.IntN(cores+2)
		full, fragment := 1+rng.IntN(3), 25*(1+rng.IntN(3))
		request := float64(full) + float64(fragment)/float64(shareBase)

		got := newHost(cpuMap, shareBase, maxFragment).getCPUPlans(request)
		want := linearSplitCPUPlans(newHost(cpuMap, shareBase, maxFragment), request)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("cores=%v maxFragment=%d request=%v: got %v, want %v", cpuMap, maxFragment, request, got, want)
		}
	}
}

func TestCountCPUPlansMatchesTheBuiltPlans(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 17))
	for range 300 {
		cores := 1 + rng.IntN(24)
		numaNodes := rng.IntN(3)
		resourceInfo := &types.NodeResourceInfo{
			Capacity: &types.NodeResource{
				CPU:        float64(cores),
				CPUMap:     types.CPUMap{},
				Memory:     int64(rng.IntN(64)) * units.GiB,
				NUMA:       types.NUMA{},
				NUMAMemory: types.NUMAMemory{},
			},
		}
		for i := range cores {
			resourceInfo.Capacity.CPUMap[strconv.Itoa(i)] = rng.IntN(4) * 25
			if numaNodes > 0 {
				resourceInfo.Capacity.NUMA[strconv.Itoa(i)] = strconv.Itoa(i % numaNodes)
			}
		}
		for i := range numaNodes {
			resourceInfo.Capacity.NUMAMemory[strconv.Itoa(i)] = resourceInfo.Capacity.Memory / int64(numaNodes)
		}
		assert.Nil(t, resourceInfo.Validate())

		maxFragment := -1 + rng.IntN(cores+2)
		req := &types.WorkloadResourceRequest{
			CPUBind:    true,
			CPURequest: float64(1+rng.IntN(3)) + float64(25*rng.IntN(4))/100,
			MemRequest: int64(rng.IntN(4)) * units.GiB,
		}
		want := len(GetCPUPlans(resourceInfo, nil, 100, maxFragment, req))
		if got := CountCPUPlans(resourceInfo, nil, 100, maxFragment, req); got != want {
			t.Fatalf("cores=%v numa=%v maxFragment=%d req=%+v: got %d, want %d", resourceInfo.Capacity.CPUMap, resourceInfo.Capacity.NUMA, maxFragment, req, got, want)
		}
	}
}

func BenchmarkGetCPUPlans(b *testing.B) {
	resourceInfo := &types.NodeResourceInfo{
		Capacity: &types.NodeResource{
			CPU:    24,
			CPUMap: types.CPUMap{},
			Memory: 128 * units.GiB,
		},
	}
	for i := range 24 {
		resourceInfo.Capacity.CPUMap[strconv.Itoa(i)] = 100
	}
	assert.Nil(b, resourceInfo.Validate())
	for b.Loop() {
		assert.True(b, len(GetCPUPlans(resourceInfo, nil, 100, -1, &types.WorkloadResourceRequest{
			CPUBind:    true,
			CPURequest: 1.3,
			MemRequest: 1,
		})) > 0)
	}
}

func applyCPUPlans(t *testing.T, resourceInfo *types.NodeResourceInfo, cpuPlans []*types.CPUPlan) {
	for _, cpuPlan := range cpuPlans {
		resourceInfo.Usage.CPUMap.Add(cpuPlan.CPUMap)
	}
	assert.Nil(t, resourceInfo.Validate())
}

func linearSplitCPUPlans(h *host, cpuRequest float64) []types.CPUMap {
	piecesRequest := int(cpuRequest * float64(h.shareBase))
	full, fragment := piecesRequest/h.shareBase, piecesRequest%h.shareBase
	maxFragmentCores := len(h.fullCores) + len(h.fragmentCores) - full
	if h.maxFragmentCores != -1 && h.maxFragmentCores < maxFragmentCores {
		maxFragmentCores = h.maxFragmentCores
	}
	if fragment == 0 || full == 0 {
		return h.getCPUPlans(cpuRequest)
	}
	totalFragmentCapacity := 0
	bestCPUPlans := [2][]types.CPUMap{h.getFullCPUPlans(h.fullCores, full), h.getFragmentCPUPlans(h.fragmentCores, fragment)}
	bestCapacity := min(len(bestCPUPlans[0]), len(bestCPUPlans[1]))
	for _, core := range h.fragmentCores {
		totalFragmentCapacity += core.pieces / fragment
	}
	for len(h.fragmentCores) < maxFragmentCores {
		newFragmentCore := h.fullCores[0]
		h.fragmentCores = append(h.fragmentCores, newFragmentCore)
		h.fullCores = h.fullCores[1:]
		totalFragmentCapacity += newFragmentCore.pieces / fragment
		fullCPUPlans := h.getFullCPUPlans(h.fullCores, full)
		if capacity := min(len(fullCPUPlans), totalFragmentCapacity); capacity > bestCapacity {
			bestCPUPlans[0] = fullCPUPlans
			bestCPUPlans[1] = h.getFragmentCPUPlans(h.fragmentCores, fragment)
			bestCapacity = capacity
		}
	}
	cpuPlans := []types.CPUMap{}
	for i := range bestCapacity {
		cpuMap := types.CPUMap{}
		cpuMap.Add(bestCPUPlans[0][i])
		cpuMap.Add(bestCPUPlans[1][i])
		cpuPlans = append(cpuPlans, cpuMap)
	}
	return cpuPlans
}
