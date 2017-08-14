package complexscheduler

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/types"
)

func resultLength(result map[string][]types.CPUMap) int {
	length := 0
	for _, list := range result {
		length += len(list)
	}
	return length
}

func newPotassium() (*potassium, error) {
	coreCfg := types.Config{
		EtcdMachines:   []string{"http://127.0.0.1:2379"},
		EtcdLockPrefix: "/eru-core/_lock",
		Scheduler: types.SchedConfig{
			LockKey:   "/coretest",
			LockTTL:   1,
			Type:      "complex",
			ShareBase: 10,
			MaxShare:  -1,
		},
	}

	potassium, err := New(coreCfg)
	if err != nil {
		return nil, fmt.Errorf("Create Potassim error: %v", err)
	}
	return potassium, nil
}

func newPod(count int) []types.NodeInfo {
	n := types.NodeInfo{
		CPUAndMem: types.CPUAndMem{
			CpuMap: types.CPUMap{
				"0": 10,
				"1": 10,
			},
			MemCap: 4 * 1024 * 1024 * 1024,
		},
		Name:    "",
		CPURate: 2e9,
	}

	nodes := []types.NodeInfo{}
	for i := 0; i < count; i++ {
		n.Name = "node" + strconv.Itoa(i)
		nodes = append(nodes, n)
	}

	return nodes
}

func TestSelectCPUNodes(t *testing.T) {
	k, _ := newPotassium()

	_, _, err := k.SelectCPUNodes([]types.NodeInfo{}, 1, 1)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "[SelectCPUNodes] No nodes provide to choose some")

	nodes := newPod(2)
	_, _, err = k.SelectCPUNodes(nodes, 0.5, 1)
	assert.NoError(t, err)
	//fmt.Printf("algorithm debug res: %v", debug)
	//fmt.Printf("algorithm debug changed: %v", changed)

	_, _, err = k.SelectCPUNodes(nodes, 2, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, _, err = k.SelectCPUNodes(nodes, 3, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, _, err = k.SelectCPUNodes(nodes, 1, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	// new round test
	nodes = newPod(2)
	r, re, err := k.SelectCPUNodes(nodes, 1, 2)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(r))
	assert.Equal(t, 2, len(re))

	for nodename, cpus := range r {
		assert.Contains(t, []string{"node0", "node1"}, nodename)
		// assert.Equal(t, len(cpus), 1)
		cpu := cpus[0]
		assert.Equal(t, cpu.Total(), int64(10))
	}

	// SelectCPUNodes 里有一些副作用, 粗暴地拿一个新的来测试吧
	// 下面也是因为这个
	nodes = newPod(2)
	r, _, err = k.SelectCPUNodes(nodes, 1.3, 2)
	assert.NoError(t, err)

	for nodename, cpus := range r {
		assert.Contains(t, []string{"node0", "node1"}, nodename)
		assert.Equal(t, len(cpus), 1)

		cpu := cpus[0]
		assert.Equal(t, cpu.Total(), int64(13))
	}
}

func checkAvgPlan(res map[string][]types.CPUMap, minCon int, maxCon int, name string) error {
	var minC int
	var maxC int
	var temp int
	for _, v := range res {
		temp = len(v)
		if minC > temp || minC == 0 {
			minC = temp
		}
		if maxC < temp {
			maxC = temp
		}
	}
	if minC != minCon || maxC != maxCon {
		//fmt.Println(name)
		//fmt.Println("min: ", minC)
		//fmt.Println("max: ", maxC)
		//for k, v := range res {
		//	fmt.Println(k, ":", len(v))
		//}
		return fmt.Errorf("alloc plan error")
	}
	return nil
}

func TestRecurrence(t *testing.T) {
	// 利用相同线上数据复现线上出现的问题

	k, _ := newPotassium()

	nodes := []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{"0": 0, "10": 0, "7": 0, "8": 10, "9": 10, "13": 0, "14": 0, "15": 10, "2": 10, "5": 10, "11": 0, "12": 0, "4": 0, "1": 0, "3": 10, "6": 0},
				MemCap: 12400000,
			},
			Name: "c2-docker-26",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{"6": 10, "10": 0, "13": 0, "14": 10, "2": 0, "7": 0, "1": 0, "11": 0, "15": 0, "8": 10, "0": 0, "3": 0, "4": 0, "5": 0, "9": 10, "12": 0},
				MemCap: 12400000,
			},
			Name: "c2-docker-27",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{"13": 0, "14": 0, "15": 0, "4": 10, "9": 0, "1": 0, "10": 0, "12": 10, "5": 10, "6": 10, "8": 10, "0": 0, "11": 0, "2": 10, "3": 0, "7": 0},
				MemCap: 12400000,
			},
			Name: "c2-docker-28",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{"15": 0, "3": 10, "0": 0, "10": 0, "13": 0, "7": 10, "8": 0, "9": 10, "12": 10, "2": 10, "4": 10, "1": 0, "11": 0, "14": 10, "5": 10, "6": 10},
				MemCap: 12400000,
			},
			Name: "c2-docker-29",
		},
	}

	_, _, err := k.SelectCPUNodes(nodes, 0.5, 3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestComplexNodes(t *testing.T) {

	coreCfg := types.Config{
		EtcdMachines:   []string{"http://127.0.0.1:2379"},
		EtcdLockPrefix: "/eru-core/_lock",
		Scheduler: types.SchedConfig{
			LockKey:   "/coretest",
			LockTTL:   1,
			Type:      "complex",
			ShareBase: 10,
			MaxShare:  -1,
		},
	}

	k, merr := New(coreCfg)
	if merr != nil {
		t.Fatalf("Create Potassim error: %v", merr)
	}

	nodes := []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 2 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
				},
				MemCap: 12400000,
			},
			Name: "n1",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 7 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
					"12": 10, "13": 10,
				},
				MemCap: 12400000,
			},
			Name: "n2",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 6 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
				},
				MemCap: 12400000,
			},
			Name: "n3",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 9 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
					"12": 10, "13": 10, "14": 10, "15": 10,
					"16": 10, "17": 10,
				},
				MemCap: 12400000,
			},
			Name: "n4",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 4 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12400000,
			},
			Name: "n5",
		},
	}

	k, _ = New(coreCfg)
	// test1
	res1, changed1, err := k.SelectCPUNodes(nodes, 1.7, 7)
	if err != nil {
		t.Fatalf("sth wrong")
	}
	if check := checkAvgPlan(res1, 1, 2, "res1"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(changed1), len(res1))

	// test2
	// SelectCPUNodes 里有一些副作用, 粗暴地拿一个新的来测试吧
	// 下面也是因为这个
	nodes = []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 2 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
				},
				MemCap: 12400000,
			},
			Name: "n1",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 7 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
					"12": 10, "13": 10,
				},
				MemCap: 12400000,
			},
			Name: "n2",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 6 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
				},
				MemCap: 12400000,
			},
			Name: "n3",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 9 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
					"12": 10, "13": 10, "14": 10, "15": 10,
					"16": 10, "17": 10,
				},
				MemCap: 12400000,
			},
			Name: "n4",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 4 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12400000,
			},
			Name: "n5",
		},
	}

	res2, changed2, err := k.SelectCPUNodes(nodes, 1.7, 11)
	if err != nil {
		t.Fatalf("something went wrong")
	}
	if check := checkAvgPlan(res2, 2, 3, "res2"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(changed2), len(res2))

	// test3
	nodes = []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 2 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
				},
				MemCap: 12400000,
			},
			Name: "n1",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 7 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
					"12": 10, "13": 10,
				},
				MemCap: 12400000,
			},
			Name: "n2",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 6 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
				},
				MemCap: 12400000,
			},
			Name: "n3",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 9 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
					"12": 10, "13": 10, "14": 10, "15": 10,
					"16": 10, "17": 10,
				},
				MemCap: 12400000,
			},
			Name: "n4",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 4 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12400000,
			},
			Name: "n5",
		},
	}

	res3, changed3, err := k.SelectCPUNodes(nodes, 1.7, 23)
	assert.NoError(t, err)
	if check := checkAvgPlan(res3, 2, 6, "res3"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(changed3), len(res3))

	// test4
	nodes = []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 2 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
				},
				MemCap: 12400000,
			},
			Name: "n1",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 7 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
					"12": 10, "13": 10,
				},
				MemCap: 12400000,
			},
			Name: "n2",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 6 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
				},
				MemCap: 12400000,
			},
			Name: "n3",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 9 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
					"12": 10, "13": 10, "14": 10, "15": 10,
					"16": 10, "17": 10,
				},
				MemCap: 12400000,
			},
			Name: "n4",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 4 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12400000,
			},
			Name: "n5",
		},
	}
	_, _, newErr := k.SelectCPUNodes(nodes, 1.6, 29)
	if newErr == nil {
		t.Fatalf("how to alloc 29 containers when you only have 28?")
	}
}

func TestEvenPlan(t *testing.T) {
	k, merr := newPotassium()
	if merr != nil {
		t.Fatalf("Create Potassim error: %v", merr)
	}

	// nodes -- n1: 2, n2: 2
	pod1 := []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{
					"0": 10, "1": 10, "2": 10, "3": 10,
				},
				MemCap: 12400000,
			},
			Name: "node1",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{
					"0": 10, "1": 10, "2": 10, "3": 10,
				},
				MemCap: 12400000,
			},
			Name: "node2",
		},
	}

	res1, rem1, err := k.SelectCPUNodes(pod1, 1.3, 2)
	if err != nil {
		t.Fatalf("sth wrong")
	}
	if check := checkAvgPlan(res1, 1, 1, "res1"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(rem1), 2)

	// nodes -- n1: 4, n2: 5, n3:6, n4: 5
	pod2 := []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 4 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12400000,
			},
			Name: "n1",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 5 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10,
				},
				MemCap: 12400000,
			},
			Name: "n2",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 6 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
				},
				MemCap: 12400000,
			},
			Name: "n3",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 5 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10,
				},
				MemCap: 12400000,
			},
			Name: "n4",
		},
	}

	res2, rem2, err := k.SelectCPUNodes(pod2, 1.7, 3)
	if check := checkAvgPlan(res2, 1, 1, "res2"); check != nil {
		t.Fatalf("something went wront")
	}
	assert.Equal(t, len(rem2), 3)

	pod3 := []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 4 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12400000,
			},
			Name: "n1",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 5 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10,
				},
				MemCap: 12400000,
			},
			Name: "n2",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 6 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
				},
				MemCap: 12400000,
			},
			Name: "n3",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 5 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10,
				},
				MemCap: 12400000,
			},
			Name: "n4",
		},
	}

	res3, rem3, err := k.SelectCPUNodes(pod3, 1.7, 8)
	if check := checkAvgPlan(res3, 2, 2, "res3"); check != nil {
		t.Fatalf("something went wront")
	}
	assert.Equal(t, len(rem3), 4)

	pod4 := []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 4 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12400000,
			},
			Name: "n1",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 5 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10,
				},
				MemCap: 12400000,
			},
			Name: "n2",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 6 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
				},
				MemCap: 12400000,
			},
			Name: "n3",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 5 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10,
				},
				MemCap: 12400000,
			},
			Name: "n4",
		},
	}

	res4, rem4, err := k.SelectCPUNodes(pod4, 1.7, 10)
	if check := checkAvgPlan(res4, 2, 3, "res4"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(rem4), 4)
}

func TestSpecialCase(t *testing.T) {
	pod := []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 4 containers
					"0": 10, "1": 10,
				},
				MemCap: 12400000,
			},
			Name: "n1",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 5 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10,
				},
				MemCap: 12400000,
			},
			Name: "n2",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 6 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12400000,
			},
			Name: "n3",
		},
	}

	k, _ := newPotassium()
	res1, _, err := k.SelectCPUNodes(pod, 1.7, 7)
	if err != nil {
		t.Fatalf("something went wrong")
	}
	checkAvgPlan(res1, 1, 3, "new test 2")

	newpod := []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 4 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10,
				},
				MemCap: 12400000,
			},
			Name: "n1",
		},
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{ // 4 containers
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12400000,
			},
			Name: "n2",
		},
	}

	res2, changed2, _ := k.SelectCPUNodes(newpod, 1.7, 4)
	assert.Equal(t, len(res2), len(changed2))
	checkAvgPlan(res2, 2, 2, "new test 2")
}

func generateNodes(nums, maxCores, seed int) []types.NodeInfo {
	var name string
	var cores int
	pod := []types.NodeInfo{}

	s := rand.NewSource(int64(seed))
	r1 := rand.New(s)

	for i := 0; i < nums; i++ {
		name = fmt.Sprintf("n%d", i)
		cores = r1.Intn(maxCores + 1)

		cpumap := types.CPUMap{}
		for j := 0; j < cores; j++ {
			coreName := fmt.Sprintf("%d", j)
			cpumap[coreName] = 10
		}
		cpuandmem := types.CPUAndMem{
			CpuMap: cpumap,
			MemCap: 12040000,
		}
		nodeInfo := types.NodeInfo{
			CPUAndMem: cpuandmem,
			Name:      name,
		}
		pod = append(pod, nodeInfo)
	}
	return pod
}

var hugePod = generateNodes(10000, 24, 10086)

func getPodVol(nodes []types.NodeInfo, cpu float64) int {
	var res int
	var host *host
	var plan []types.CPUMap

	for _, nodeInfo := range nodes {
		host = newHost(nodeInfo.CPUAndMem.CpuMap, 10)
		plan = host.getContainerCores(cpu, -1)
		res += len(plan)
	}
	return res
}

func TestGetPodVol(t *testing.T) {
	nodes := []types.NodeInfo{
		types.NodeInfo{
			CPUAndMem: types.CPUAndMem{
				CpuMap: types.CPUMap{"15": 0, "3": 10, "0": 0, "10": 0, "13": 0, "7": 10, "8": 0, "9": 10, "12": 10, "2": 10, "4": 10, "1": 0, "11": 0, "14": 10, "5": 10, "6": 10},
				MemCap: 12400000,
			},
			Name: "c2-docker-26",
		},
	}

	res := getPodVol(nodes, 0.5)
	assert.Equal(t, res, 18)
	res = getPodVol(nodes, 0.3)
	assert.Equal(t, res, 27)
	res = getPodVol(nodes, 1.1)
	assert.Equal(t, res, 8)
}

func Benchmark_ExtreamAlloc(b *testing.B) {
	k, _ := newPotassium()
	b.StopTimer()
	b.StartTimer()

	vol := getPodVol(hugePod, 1.3)
	result, changed, err := k.SelectCPUNodes(hugePod, 1.3, vol)
	if err != nil {
		b.Fatalf("something went wrong")
	}
	assert.Equal(b, len(result), len(changed))
}

func Benchmark_AveAlloc(b *testing.B) {
	b.StopTimer()
	k, _ := newPotassium()

	b.StartTimer()
	result, changed, err := k.SelectCPUNodes(hugePod, 1.7, 12000)
	if err != nil {
		b.Fatalf("something went wrong")
	}
	assert.Equal(b, len(result), len(changed))
}

// Test SelectMemoryNodes
func TestSelectMemoryNodes(t *testing.T) {
	// 2 nodes [2 containers per node]
	pod := newPod(2)
	k, _ := newPotassium()
	res, err := k.SelectMemoryNodes(pod, 10000, 512*1024*1024, 4)
	assert.NoError(t, err)
	for _, node := range res {
		assert.Equal(t, node.Deploy, 2)
	}

	// 4 nodes [1 container per node]
	pod = newPod(4)
	res, err = k.SelectMemoryNodes(pod, 10000, 512*1024*1024, 1)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy, 1)

	// 4 nodes [1 container per node]
	pod = newPod(4)
	res, err = k.SelectMemoryNodes(pod, 10000, 512*1024*1024, 4)
	assert.NoError(t, err)
	for _, node := range res {
		assert.Equal(t, node.Deploy, 1)
	}

	// 4 nodes
	pod = newPod(4)
	for i := 0; i < 4; i++ {
		pod[i].Count += i
	}
	res, err = k.SelectMemoryNodes(pod, 10000, 512*1024*1024, 6)
	assert.NoError(t, err)
	for i, node := range res {
		fmt.Printf("[%v] already: [%v], plan: [%v], final: [%v]\n", node.Name, node.Count, node.Deploy, node.Count+node.Deploy)
		assert.Equal(t, node.Deploy, 3-i)
	}
}

func TestTestSelectMemoryNodesNotEnough(t *testing.T) {
	// 2 nodes [mem not enough]
	pod := newPod(2)
	k, _ := newPotassium()
	_, err := k.SelectMemoryNodes(pod, 10000, 512*1024*1024, 40)
	assert.Error(t, err)
	if err != nil {
		assert.Equal(t, err.Error(), "[SelectMemoryNodes] Cannot alloc a plan, not enough memory, volume 16, need 40")
	}

	// 2 nodes [mem not enough]
	pod = newPod(2)
	_, err = k.SelectMemoryNodes(pod, 1e9, 5*1024*1024*1024, 1)
	assert.Error(t, err)
	if err != nil {
		assert.Equal(t, err.Error(), "[SelectMemoryNodes] Cannot alloc a plan, not enough memory, volume 0, need 1")
	}

	// 2 nodes [cpu not enough]
	pod = newPod(2)
	_, err = k.SelectMemoryNodes(pod, 1e10, 512*1024*1024, 1)
	assert.Error(t, err)
	if err != nil {
		assert.Equal(t, err.Error(), "[SelectMemoryNodes] Cannot alloc a plan, not enough cpu rate")
	}
}

func refreshPod(nodes []types.NodeInfo) {
	for i := range nodes {
		nodes[i].Count += nodes[i].Deploy
		nodes[i].MemCap -= int64(nodes[i].Deploy * 512 * 1024 * 1024)
		nodes[i].Deploy = 0
	}
}

func TestSelectMemoryNodesSequence(t *testing.T) {
	pod := newPod(2)
	k, _ := newPotassium()
	res, err := k.SelectMemoryNodes(pod, 10000, 512*1024*1024, 1)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node0" {
			assert.Equal(t, node.Deploy, 1)
		}
	}

	refreshPod(res)
	res, err = k.SelectMemoryNodes(res, 10000, 512*1024*1024, 1)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node1" {
			assert.Equal(t, node.Deploy, 1)
		}
	}

	refreshPod(res)
	res, err = k.SelectMemoryNodes(res, 10000, 512*1024*1024, 4)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy, 2)
	assert.Equal(t, res[1].Deploy, 2)

	refreshPod(res)
	res, err = k.SelectMemoryNodes(res, 10000, 512*1024*1024, 3)
	assert.NoError(t, err)
	for _, node := range res {
		fmt.Printf("[%v] already: [%v], plan: [%v], final: [%v]\n", node.Name, node.Count, node.Deploy, node.Count+node.Deploy)
	}
	assert.Equal(t, res[0].Deploy+res[1].Deploy, 3)
	assert.Equal(t, res[0].Deploy-res[1].Deploy, 1)

	refreshPod(res)
	res, err = k.SelectMemoryNodes(res, 10000, 512*1024*1024, 40)
	assert.Error(t, err)
	if err != nil {
		assert.Equal(t, err.Error(), "[SelectMemoryNodes] Cannot alloc a plan, not enough memory, volume 7, need 40")
	}

	// new round
	pod = newPod(2)
	res, err = k.SelectMemoryNodes(pod, 10000, 512*1024*1024, 1)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node0" {
			assert.Equal(t, node.Deploy, 1)
		}
	}
	refreshPod(res)
	res, err = k.SelectMemoryNodes(res, 10000, 512*1024*1024, 2)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node1" {
			assert.Equal(t, node.Deploy, 2)
		}
	}
	refreshPod(res)
	res, err = k.SelectMemoryNodes(res, 10000, 512*1024*1024, 5)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy+res[0].Count, 4)
	assert.Equal(t, res[1].Deploy+res[1].Count, 4)
}

func TestSelectMemoryNodesGiven(t *testing.T) {
	pod := newPod(4)
	for i := 0; i < 3; i++ {
		pod[i].Count++
	}

	k, _ := newPotassium()
	res, err := k.SelectMemoryNodes(pod, 10000, 512*1024*1024, 2)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node3" {
			assert.Equal(t, node.Deploy, 2)
		} else {
			assert.Equal(t, node.Deploy, 0)
		}
		// fmt.Printf("[%v] already: [%v], plan: [%v], final: [%v]\n", node.Name, node.Count, node.Deploy, node.Count+node.Deploy)
	}
}
