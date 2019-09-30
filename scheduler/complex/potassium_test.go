package complexscheduler

import (
	"errors"
	"fmt"
	"testing"

	"math"

	"github.com/docker/go-units"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func newConfig() types.Config {
	return types.Config{
		Etcd: types.EtcdConfig{
			Machines:   []string{"http://127.0.0.1:2379"},
			LockPrefix: "core/_lock",
		},
		Scheduler: types.SchedConfig{
			ShareBase: 10,
			MaxShare:  -1,
		},
	}
}

func newPotassium() (*Potassium, error) {
	coreCfg := newConfig()
	potassium, err := New(coreCfg)
	if err != nil {
		return nil, fmt.Errorf("Create Potassim error: %v", err)
	}
	return potassium, nil
}

func generateNodes(nums, cores int, memory, storage int64, shares int) []types.NodeInfo {
	var name string
	nodes := []types.NodeInfo{}

	for i := 0; i < nums; i++ {
		name = fmt.Sprintf("n%d", i)

		cpumap := types.CPUMap{}
		for j := 0; j < cores; j++ {
			coreName := fmt.Sprintf("%d", j)
			cpumap[coreName] = shares
		}
		nodeInfo := types.NodeInfo{
			CPUMap:     cpumap,
			MemCap:     memory,
			StorageCap: storage,
			Name:       name,
		}
		nodes = append(nodes, nodeInfo)
	}
	return nodes
}

func getNodesCapacity(nodes []types.NodeInfo, cpu float64, shares, maxshare int) int {
	var res int
	var host *host
	var plan []types.CPUMap

	for _, nodeInfo := range nodes {
		host = newHost(nodeInfo.CPUMap, shares)
		plan = host.getContainerCores(cpu, maxshare)
		res += len(plan)
	}
	return res
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
		return fmt.Errorf("alloc plan error")
	}
	return nil
}

func refreshPod(nodes []types.NodeInfo, memory, storage int64) {
	for i := range nodes {
		nodes[i].Count += nodes[i].Deploy
		nodes[i].MemCap -= int64(nodes[i].Deploy) * memory
		nodes[i].StorageCap -= int64(nodes[i].Deploy) * storage
		nodes[i].Deploy = 0
	}
}

func getComplexNodes() []types.NodeInfo {
	return []types.NodeInfo{
		{
			CPUMap: types.CPUMap{ // 2 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n1",
		},
		{
			CPUMap: types.CPUMap{ // 7 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10, "6": 10, "7": 10,
				"8": 10, "9": 10, "10": 10, "11": 10,
				"12": 10, "13": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n2",
		},
		{
			CPUMap: types.CPUMap{ // 6 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10, "6": 10, "7": 10,
				"8": 10, "9": 10, "10": 10, "11": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n3",
		},
		{
			CPUMap: types.CPUMap{ // 9 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10, "6": 10, "7": 10,
				"8": 10, "9": 10, "10": 10, "11": 10,
				"12": 10, "13": 10, "14": 10, "15": 10,
				"16": 10, "17": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n4",
		},
		{
			CPUMap: types.CPUMap{ // 4 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10, "6": 10, "7": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n5",
		},
	}
}

func getEvenPlanNodes() []types.NodeInfo {
	return []types.NodeInfo{
		{
			CPUMap: types.CPUMap{ // 4 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10, "6": 10, "7": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n1",
		},
		{
			CPUMap: types.CPUMap{ // 5 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10, "6": 10, "7": 10,
				"8": 10, "9": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n2",
		},
		{
			CPUMap: types.CPUMap{ // 6 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10, "6": 10, "7": 10,
				"8": 10, "9": 10, "10": 10, "11": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n3",
		},
		{
			CPUMap: types.CPUMap{ // 5 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10, "6": 10, "7": 10,
				"8": 10, "9": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n4",
		},
	}
}

func SelectCPUNodes(k *Potassium, nodesInfo []types.NodeInfo, quota float64, memory int64, need int, each bool) (map[string][]types.CPUMap, map[string]types.CPUMap, error) {
	nodesInfo, nodePlans, total, err := k.SelectCPUNodes(nodesInfo, quota, memory)
	if err != nil {
		return nil, nil, err
	}

	if each {
		nodesInfo, err = k.EachDivision(nodesInfo, need, 0)
	} else {
		nodesInfo, err = k.CommonDivision(nodesInfo, need, total)
	}

	if err != nil {
		return nil, nil, err
	}

	result := make(map[string][]types.CPUMap)
	changed := make(map[string]types.CPUMap)

	// 只返回有修改的就可以了, 返回有修改的还剩下多少
	for _, nodeInfo := range nodesInfo {
		if nodeInfo.Deploy <= 0 {
			continue
		}
		cpuList := nodePlans[nodeInfo.Name][:nodeInfo.Deploy]
		result[nodeInfo.Name] = cpuList
		for _, cpu := range cpuList {
			nodeInfo.CPUMap.Sub(cpu)
		}
		changed[nodeInfo.Name] = nodeInfo.CPUMap
	}
	return result, changed, nil
}

func SelectMemoryNodes(k *Potassium, nodesInfo []types.NodeInfo, rate float64, memory int64, need int, each bool) ([]types.NodeInfo, error) {
	nodesInfo, total, err := k.SelectMemoryNodes(nodesInfo, rate, memory)
	if err != nil {
		return nodesInfo, err
	}
	// Make deploy plan
	if each {
		nodesInfo, err = k.EachDivision(nodesInfo, need, 0)
	} else {
		nodesInfo, err = k.CommonDivision(nodesInfo, need, total)
	}
	return nodesInfo, err
}

func TestSelectCPUNodes(t *testing.T) {
	k, _ := newPotassium()
	memory := 4 * int64(units.GiB)

	_, _, err := SelectCPUNodes(k, []types.NodeInfo{}, 1, 1, 1, false)
	assert.True(t, errors.Is(err, types.ErrZeroNodes))

	nodes := generateNodes(2, 2, memory, 0, 10)
	_, _, err = SelectCPUNodes(k, nodes, 0.5, 1, 1, false)
	assert.NoError(t, err)

	_, _, err = SelectCPUNodes(k, nodes, 2, 1, 3, false)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))
	assert.Contains(t, err.Error(), "need: 3, vol: 1")

	_, _, err = SelectCPUNodes(k, nodes, 3, 1, 2, false)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))

	_, _, err = SelectCPUNodes(k, nodes, 1, 1, 5, false)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))

	// new round test
	nodes = generateNodes(2, 2, memory, 0, 10)
	r, re, err := SelectCPUNodes(k, nodes, 1, 1, 2, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(r))
	assert.Equal(t, 2, len(re))

	for nodename, cpus := range r {
		assert.Contains(t, []string{"n0", "n1"}, nodename)
		// assert.Equal(t, len(cpus), 1)
		cpu := cpus[0]
		assert.Equal(t, cpu.Total(), 10)
	}

	// SelectCPUNodes 里有一些副作用, 粗暴地拿一个新的来测试吧
	// 下面也是因为这个
	nodes = generateNodes(2, 2, memory, 0, 10)
	r, _, err = SelectCPUNodes(k, nodes, 1.3, 1, 2, false)
	assert.NoError(t, err)

	for nodename, cpus := range r {
		assert.Contains(t, []string{"n0", "n1"}, nodename)
		assert.Equal(t, len(cpus), 1)

		cpu := cpus[0]
		assert.Equal(t, cpu.Total(), 13)
	}
}

func TestSelectCPUNodesWithMemoryLimit(t *testing.T) {
	k, _ := newPotassium()

	// 测试 2 个 Node，每个 CPU 10%，但是内存吃满
	nodes := generateNodes(2, 2, 1024, 0, 10)
	result, _, err := SelectCPUNodes(k, nodes, 0.1, 1024, 1, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result), 2)
	for _, cpus := range result {
		assert.Equal(t, len(cpus), 1)
	}

	// 测试 2 个 Node，内存不足
	nodes = generateNodes(2, 2, 1024, 0, 10)
	_, _, err = SelectCPUNodes(k, nodes, 0.1, 1025, 1, true)
	assert.EqualError(t, err, types.ErrInsufficientRes.Error())

	// 测试 need 超过 each node 的 capacity
	nodes = generateNodes(2, 2, 1024, 0, 10)
	_, _, err = SelectCPUNodes(k, nodes, 0.1, 1024, 2, true)
	assert.EqualError(t, err, types.ErrInsufficientCap.Error())
}

func TestRecurrence(t *testing.T) {
	// 利用相同线上数据复现线上出现的问题

	k, _ := newPotassium()

	nodes := []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 0, "10": 0, "7": 0, "8": 10, "9": 10, "13": 0, "14": 0, "15": 10, "2": 10, "5": 10, "11": 0, "12": 0, "4": 0, "1": 0, "3": 10, "6": 0},
			MemCap: 12 * int64(units.GiB),
			Name:   "c2-node-26",
		},
		{
			CPUMap: types.CPUMap{"6": 10, "10": 0, "13": 0, "14": 10, "2": 0, "7": 0, "1": 0, "11": 0, "15": 0, "8": 10, "0": 0, "3": 0, "4": 0, "5": 0, "9": 10, "12": 0},
			MemCap: 12 * int64(units.GiB),
			Name:   "c2-node-27",
		},
		{
			CPUMap: types.CPUMap{"13": 0, "14": 0, "15": 0, "4": 10, "9": 0, "1": 0, "10": 0, "12": 10, "5": 10, "6": 10, "8": 10, "0": 0, "11": 0, "2": 10, "3": 0, "7": 0},
			MemCap: 12 * int64(units.GiB),
			Name:   "c2-node-28",
		},
		{
			CPUMap: types.CPUMap{"15": 0, "3": 10, "0": 0, "10": 0, "13": 0, "7": 10, "8": 0, "9": 10, "12": 10, "2": 10, "4": 10, "1": 0, "11": 0, "14": 10, "5": 10, "6": 10},
			MemCap: 12 * int64(units.GiB),
			Name:   "c2-node-29",
		},
	}

	r, rp, total, err := k.SelectCPUNodes(nodes, 0.5, 1)
	assert.NoError(t, err)
	assert.Equal(t, len(r), len(rp))
	v := 0
	for _, cpus := range rp {
		v += len(cpus)
	}
	assert.Equal(t, v, total)
}

func TestComplexNodes(t *testing.T) {
	coreCfg := newConfig()

	k, merr := New(coreCfg)
	if merr != nil {
		t.Fatalf("Create Potassim error: %v", merr)
	}

	// test1
	nodes := getComplexNodes()
	res1, changed1, err := SelectCPUNodes(k, nodes, 1.7, 1, 7, false)
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
	nodes = getComplexNodes()
	res2, changed2, err := SelectCPUNodes(k, nodes, 1.7, 1, 11, false)
	if err != nil {
		t.Fatalf("something went wrong")
	}
	if check := checkAvgPlan(res2, 2, 3, "res2"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(changed2), len(res2))

	// test3
	nodes = getComplexNodes()
	res3, changed3, err := SelectCPUNodes(k, nodes, 1.7, 1, 23, false)
	assert.NoError(t, err)
	if check := checkAvgPlan(res3, 2, 6, "res3"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(changed3), len(res3))

	// test4
	nodes = getComplexNodes()
	_, _, newErr := SelectCPUNodes(k, nodes, 1.6, 1, 29, false)
	if newErr == nil {
		t.Fatalf("how to alloc 29 containers when you only have 28?")
	}

	//test5
	nodes = getComplexNodes()
	_, _, err = SelectCPUNodes(k, nodes, 0, 1, 10, false)
	assert.EqualError(t, err, types.ErrNegativeQuota.Error())

	//test6
	nodes = getComplexNodes()
	res6, _, err := SelectCPUNodes(k, nodes, 1, 1, 2, true)
	assert.NoError(t, err)
	assert.Equal(t, len(res6), 5)
}

func TestCPUWithMaxShareLimit(t *testing.T) {
	coreCfg := newConfig()
	coreCfg.Scheduler.ShareBase = 100
	coreCfg.Scheduler.MaxShare = 2
	k, err := New(coreCfg)
	if err != nil {
		t.Fatalf("Create Potassim error: %v", err)
	}

	// oversell
	nodes := []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100, "4": 100, "5": 100},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}

	_, _, err = SelectCPUNodes(k, nodes, 1.7, 1, 3, false)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))
	assert.Contains(t, err.Error(), "vol: 2")
}

func TestCpuOverSell(t *testing.T) {
	coreCfg := newConfig()
	coreCfg.Scheduler.ShareBase = 100
	k, err := New(coreCfg)
	if err != nil {
		t.Fatalf("Create Potassim error: %v", err)
	}

	// oversell
	nodes := []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 300, "1": 300},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}

	r, c, err := SelectCPUNodes(k, nodes, 2, 1, 3, false)
	assert.NoError(t, err)
	assert.Equal(t, r["nodes1"][0]["0"], 100)
	assert.Equal(t, r["nodes1"][0]["1"], 100)
	assert.Equal(t, c["nodes1"]["0"], 0)
	assert.Equal(t, c["nodes1"]["1"], 0)

	// oversell fragment
	nodes = []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 300},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}

	_, _, err = SelectCPUNodes(k, nodes, 0.5, 1, 6, false)
	assert.NoError(t, err)

	// one core oversell
	nodes = []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 300},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}

	_, _, err = SelectCPUNodes(k, nodes, 1, 1, 2, false)
	assert.NoError(t, err)

	// balance
	nodes = []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 100, "1": 200, "2": 300},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}
	_, c, err = SelectCPUNodes(k, nodes, 1, 1, 2, false)
	assert.NoError(t, err)
	assert.Equal(t, c["nodes1"]["0"], 0)
	assert.Equal(t, c["nodes1"]["1"], 100)

	// complex
	nodes = []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 50, "1": 100, "2": 300, "3": 70, "4": 200, "5": 30, "6": 230},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}
	_, _, err = SelectCPUNodes(k, nodes, 1.7, 1, 2, false)
	assert.NoError(t, err)

	nodes = []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 70, "1": 100, "2": 400},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}
	_, c, err = SelectCPUNodes(k, nodes, 1.3, 1, 4, false)
	assert.NoError(t, err)
	assert.Equal(t, c["nodes1"]["0"], 10)
	assert.Equal(t, c["nodes1"]["1"], 40)
	assert.Equal(t, c["nodes1"]["2"], 0)
}

func TestCPUOverSellAndStableFragmentCore(t *testing.T) {
	coreCfg := newConfig()
	coreCfg.Scheduler.ShareBase = 100
	k, err := New(coreCfg)
	if err != nil {
		t.Fatalf("Create Potassim error: %v", err)
	}

	// oversell
	nodes := []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 300, "1": 300},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}

	_, _, err = SelectCPUNodes(k, nodes, 1.7, 1, 1, false)
	assert.NoError(t, err)

	// stable fragment core
	nodes = []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 230, "1": 200},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}
	_, changed, err := SelectCPUNodes(k, nodes, 1.7, 1, 1, false)
	assert.NoError(t, err)
	assert.Equal(t, changed["nodes1"]["0"], 160)
	nodes[0].CPUMap = changed["nodes1"]
	nodes[0].Deploy = 0
	nodes[0].Count = 0
	nodes[0].Capacity = 0
	_, changed, err = SelectCPUNodes(k, nodes, 0.3, 1, 1, false)
	assert.NoError(t, err)
	assert.Equal(t, changed["nodes1"]["0"], 130)
	assert.Equal(t, changed["nodes1"]["1"], 100)

	// complex node
	nodes = []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 230, "1": 80, "2": 300, "3": 200},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}
	_, changed, err = SelectCPUNodes(k, nodes, 1.7, 1, 2, false)
	assert.NoError(t, err)
	assert.Equal(t, changed["nodes1"]["0"], 160)
	assert.Equal(t, changed["nodes1"]["1"], 10)

	// consume full core
	nodes = []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 70, "1": 50, "2": 100, "3": 100, "4": 100},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}
	_, changed, err = SelectCPUNodes(k, nodes, 1.7, 1, 2, false)
	assert.NoError(t, err)
	assert.Equal(t, changed["nodes1"]["0"], 0)
	assert.Equal(t, changed["nodes1"]["1"], 50)

	// consume less fragment core
	nodes = []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"0": 70, "1": 50, "2": 90},
			MemCap: 12 * int64(units.GiB),
			Name:   "nodes1",
		},
	}
	_, changed, err = SelectCPUNodes(k, nodes, 0.5, 1, 2, false)
	assert.NoError(t, err)
	assert.Equal(t, changed["nodes1"]["0"], 20)
	assert.Equal(t, changed["nodes1"]["1"], 0)
	assert.Equal(t, changed["nodes1"]["2"], 90)
}

func TestEvenPlan(t *testing.T) {
	k, merr := newPotassium()
	if merr != nil {
		t.Fatalf("Create Potassim error: %v", merr)
	}

	// nodes -- n1: 2, n2: 2
	pod1 := []types.NodeInfo{
		{
			CPUMap: types.CPUMap{
				"0": 10, "1": 10, "2": 10, "3": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "node1",
		},
		{
			CPUMap: types.CPUMap{
				"0": 10, "1": 10, "2": 10, "3": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "node2",
		},
	}

	res1, rem1, err := SelectCPUNodes(k, pod1, 1.3, 1, 2, false)
	if err != nil {
		t.Fatalf("sth wrong")
	}
	if check := checkAvgPlan(res1, 1, 1, "res1"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(rem1), 2)

	// nodes -- n1: 4, n2: 5, n3:6, n4: 5
	pod2 := getEvenPlanNodes()
	res2, rem2, _ := SelectCPUNodes(k, pod2, 1.7, 1, 3, false)
	if check := checkAvgPlan(res2, 1, 1, "res2"); check != nil {
		t.Fatalf("something went wront")
	}
	assert.Equal(t, len(rem2), 3)

	pod3 := getEvenPlanNodes()
	res3, rem3, _ := SelectCPUNodes(k, pod3, 1.7, 1, 8, false)
	if check := checkAvgPlan(res3, 2, 2, "res3"); check != nil {
		t.Fatalf("something went wront")
	}
	assert.Equal(t, len(rem3), 4)

	pod4 := getEvenPlanNodes()
	res4, rem4, _ := SelectCPUNodes(k, pod4, 1.7, 1, 10, false)
	if check := checkAvgPlan(res4, 2, 3, "res4"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(rem4), 4)
}

func TestSpecialCase(t *testing.T) {
	pod := []types.NodeInfo{
		{
			CPUMap: types.CPUMap{ // 4 containers
				"0": 10, "1": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n1",
		},
		{
			CPUMap: types.CPUMap{ // 5 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n2",
		},
		{
			CPUMap: types.CPUMap{ // 6 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10, "6": 10, "7": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n3",
		},
	}

	k, _ := newPotassium()
	res1, _, err := SelectCPUNodes(k, pod, 1.7, 1, 7, false)
	assert.NoError(t, err)
	checkAvgPlan(res1, 1, 3, "new test 2")

	newpod := []types.NodeInfo{
		{
			CPUMap: types.CPUMap{ // 4 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n1",
		},
		{
			CPUMap: types.CPUMap{ // 4 containers
				"0": 10, "1": 10, "2": 10, "3": 10,
				"4": 10, "5": 10, "6": 10, "7": 10,
			},
			MemCap: 12 * int64(units.GiB),
			Name:   "n2",
		},
	}

	res2, changed2, err := SelectCPUNodes(k, newpod, 1.7, 1, 4, false)
	assert.NoError(t, err)
	assert.Equal(t, len(res2), len(changed2))
	checkAvgPlan(res2, 2, 2, "new test 2")
}

func TestGetPodVol(t *testing.T) {
	nodes := []types.NodeInfo{
		{
			CPUMap: types.CPUMap{"15": 0, "3": 10, "0": 0, "10": 0, "13": 0, "7": 10, "8": 0, "9": 10, "12": 10, "2": 10, "4": 10, "1": 0, "11": 0, "14": 10, "5": 10, "6": 10},
			MemCap: 12 * int64(units.GiB),
			Name:   "c2-node-26",
		},
	}

	res := getNodesCapacity(nodes, 0.5, 10, -1)
	assert.Equal(t, res, 18)
	res = getNodesCapacity(nodes, 0.3, 10, -1)
	assert.Equal(t, res, 27)
	res = getNodesCapacity(nodes, 1.1, 10, -1)
	assert.Equal(t, res, 8)
}

// Benchmark CPU Alloc
func Benchmark_CPUAlloc(b *testing.B) {
	b.StopTimer()
	k, _ := newPotassium()
	var cpu = 1.3
	var count = 10000
	for i := 0; i < b.N; i++ {
		// 24 core, 128G memory, 10 pieces per core
		hugePod := generateNodes(count, 24, 128*int64(units.GiB), 0, 10)
		need := getNodesCapacity(hugePod, cpu, 10, -1)
		b.StartTimer()
		r, c, err := SelectCPUNodes(k, hugePod, cpu, 1, need, false)
		b.StopTimer()
		assert.NoError(b, err)
		assert.Equal(b, len(r), len(c))
	}
}

// Benchmark Memory Alloc
func Benchmark_MemAlloc(b *testing.B) {
	b.StopTimer()
	k, _ := newPotassium()
	var count = 10000
	// 128M per container
	var memory int64 = 1024 * 1024 * 128
	// Max vol is 128G/128M * 10000 nodes
	var need = 10240000
	for i := 0; i < b.N; i++ {
		// 24 core, 128G memory, 10 pieces per core
		hugePod := generateNodes(count, 24, 128*int64(units.GiB), 0, 10)
		b.StartTimer()
		r, err := SelectMemoryNodes(k, hugePod, 1, memory, need, false)
		b.StopTimer()
		assert.NoError(b, err)
		assert.Equal(b, len(r), count)
	}
}

// Test SelectMemoryNodes
func TestSelectMemoryNodes(t *testing.T) {
	// 2 nodes [2 containers per node]
	memory := 4 * int64(units.GiB)
	pod := generateNodes(2, 2, memory, 0, 10)
	k, _ := newPotassium()
	cpus := 1.0
	res, err := SelectMemoryNodes(k, pod, cpus, 512*int64(units.MiB), 4, false)
	assert.NoError(t, err)
	for _, node := range res {
		assert.Equal(t, node.Deploy, 2)
	}

	// 4 nodes [1 container on the first node]
	pod = generateNodes(4, 2, memory, 0, 10)
	res, err = SelectMemoryNodes(k, pod, cpus, 512*int64(units.MiB), 1, false)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy, 1)

	// 4 nodes [1 container per node]
	pod = generateNodes(4, 2, memory, 0, 10)
	res, err = SelectMemoryNodes(k, pod, cpus, 512*int64(units.MiB), 4, false)
	assert.NoError(t, err)
	for _, node := range res {
		assert.Equal(t, node.Deploy, 1)
	}

	// 4 nodes
	pod = generateNodes(4, 2, memory, 0, 10)
	for i := 0; i < 4; i++ {
		pod[i].Count += i
	}
	res, err = SelectMemoryNodes(k, pod, cpus, 512*int64(units.MiB), 6, false)
	assert.NoError(t, err)
	for i, node := range res {
		assert.Equal(t, node.Deploy, 3-i)
	}

	pod = generateNodes(1, 2, memory, 0, 10)
	_, err = SelectMemoryNodes(k, pod, cpus, 0, 10, false)
	assert.EqualError(t, err, types.ErrNegativeMemory.Error())

	// test each
	pod = generateNodes(4, 2, memory, 0, 10)
	each := 2
	res, _ = SelectMemoryNodes(k, pod, 1000, 1024, each, true)
	for i := range res {
		assert.Equal(t, res[i].Deploy, each)
	}
}

func TestSelectMemoryNodesNotEnough(t *testing.T) {
	memory := 4 * int64(units.MiB)
	// 2 nodes [memory not enough]
	pod := generateNodes(2, 2, 4*int64(units.GiB), 0, 10)
	k, _ := newPotassium()
	_, err := SelectMemoryNodes(k, pod, 1, 512*int64(units.MiB), 40, false)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))
	assert.Contains(t, err.Error(), "need: 40, vol: 16")

	// 2 nodes [memory not enough]
	pod = generateNodes(2, 2, memory, 0, 10)
	_, err = SelectMemoryNodes(k, pod, 1, 5*int64(units.GiB), 1, false)
	assert.Equal(t, err, types.ErrInsufficientMEM)

	// 2 nodes [cpu not enough]
	pod = generateNodes(2, 2, memory, 0, 10)
	_, err = SelectMemoryNodes(k, pod, 1e10, 512*int64(units.MiB), 1, false)
	assert.Equal(t, err, types.ErrInsufficientCPU)
}

func TestSelectMemoryNodesSequence(t *testing.T) {
	pod := generateNodes(2, 2, 4*int64(units.GiB), 0, 10)
	k, _ := newPotassium()
	cpu := 1.0
	mem := 512 * int64(units.MiB)
	res, err := SelectMemoryNodes(k, pod, cpu, mem, 1, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node0" {
			assert.Equal(t, node.Deploy, 1)
		}
	}

	refreshPod(res, mem, 0)
	res, err = SelectMemoryNodes(k, res, cpu, mem, 1, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node1" {
			assert.Equal(t, node.Deploy, 1)
		}
	}

	refreshPod(res, mem, 0)
	res, err = SelectMemoryNodes(k, res, cpu, mem, 4, false)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy, 2)
	assert.Equal(t, res[1].Deploy, 2)

	refreshPod(res, mem, 0)
	res, err = SelectMemoryNodes(k, res, cpu, mem, 3, false)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy+res[1].Deploy, 3)
	assert.Equal(t, res[0].Deploy-res[1].Deploy, 1)

	refreshPod(res, mem, 0)
	_, err = SelectMemoryNodes(k, res, cpu, mem, 40, false)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))
	assert.Contains(t, err.Error(), "need: 40, vol: 7")

	// new round
	pod = generateNodes(2, 2, 4*int64(units.GiB), 0, 10)
	res, err = SelectMemoryNodes(k, pod, cpu, mem, 1, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node0" {
			assert.Equal(t, node.Deploy, 1)
		}
	}
	refreshPod(res, mem, 0)
	res, err = SelectMemoryNodes(k, res, cpu, mem, 2, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node1" {
			assert.Equal(t, node.Deploy, 2)
		}
	}
	refreshPod(res, mem, 0)
	res, err = SelectMemoryNodes(k, res, cpu, mem, 5, false)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy+res[0].Count, 4)
	assert.Equal(t, res[1].Deploy+res[1].Count, 4)
}

func TestSelectMemoryNodesGiven(t *testing.T) {
	pod := generateNodes(4, 2, 4*int64(units.GiB), 0, 10)
	for i := 0; i < 3; i++ {
		pod[i].Count++
	}

	k, _ := newPotassium()
	res, err := SelectMemoryNodes(k, pod, 1.0, 512*int64(units.MiB), 2, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "n3" {
			assert.Equal(t, node.Deploy, 2)
			continue
		}
		assert.Equal(t, node.Deploy, 0)
	}
}

func TestMaxIdleNode(t *testing.T) {
	n1 := &types.Node{
		Name:       "n1",
		CPU:        types.CPUMap{"0": 20},
		InitCPU:    types.CPUMap{"0": 100},
		MemCap:     30,
		InitMemCap: 100,
	}
	n2 := &types.Node{
		Name:       "n1",
		CPU:        types.CPUMap{"0": 30},
		InitCPU:    types.CPUMap{"0": 100},
		MemCap:     10,
		InitMemCap: 100,
	}
	k, _ := newPotassium()
	_, err := k.MaxIdleNode([]*types.Node{})
	assert.Error(t, err)
	node, err := k.MaxIdleNode([]*types.Node{n1, n2})
	assert.NoError(t, err)
	assert.Equal(t, node.Name, n2.Name)
}

func TestGlobalDivision(t *testing.T) {
	k, _ := newPotassium()
	_, err := k.GlobalDivision([]types.NodeInfo{}, 10, 1)
	assert.Error(t, err)
	nodeInfo := types.NodeInfo{
		Name:     "n1",
		CPUUsed:  0.7,
		MemUsage: 0.3,
		CPURate:  0.1,
		MemRate:  0.2,
		Capacity: 100,
		Count:    21,
		Deploy:   0,
	}
	r, err := k.GlobalDivision([]types.NodeInfo{nodeInfo}, 10, 100)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 10)
}

func TestSelectStorageNodesMultipleDeployedPerNode(t *testing.T) {
	k, _ := newPotassium()
	emptyNode := []types.NodeInfo{}
	_, r, err := k.SelectStorageNodes(emptyNode, -1)
	assert.Zero(t, r)
	assert.Error(t, err)
	_, r, err = k.SelectStorageNodes(emptyNode, 0)
	assert.Equal(t, r, math.MaxInt32)
	assert.NoError(t, err)
	nodesInfo := generateNodes(2, 2, 4*int64(units.GiB), 8*int64(units.GiB), 10)
	nodesInfo, total, err := k.SelectMemoryNodes(nodesInfo, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 8, total)
	assert.Equal(t, 2, len(nodesInfo))
	assert.Equal(t, 4, nodesInfo[0].Capacity)
	assert.Equal(t, 4, nodesInfo[1].Capacity)

	res, err := SelectStorageNodes(k, nodesInfo, int64(units.GiB), 4, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 2, res[0].Deploy)
	assert.Equal(t, 2, res[1].Deploy)
	assert.Equal(t, 2, res[0].Capacity)
	assert.Equal(t, 2, res[1].Capacity)
}

func TestSelectStorageNodesDeployedOnFirstNode(t *testing.T) {
	k, _ := newPotassium()
	nodesInfo := generateNodes(2, 2, 4*int64(units.GiB), int64(units.GiB), 10)
	nodesInfo, total, err := k.SelectMemoryNodes(nodesInfo, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 8, total)
	assert.Equal(t, 2, len(nodesInfo))
	assert.Equal(t, 4, nodesInfo[0].Capacity)
	assert.Equal(t, 4, nodesInfo[1].Capacity)

	res, err := SelectStorageNodes(k, nodesInfo, int64(units.MiB), 1, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 1, res[0].Deploy)
	assert.Equal(t, 0, res[1].Deploy)
	assert.Equal(t, 3, res[0].Capacity)
	assert.Equal(t, 4, res[1].Capacity)
}

func TestSelectStorageNodesOneDeployedPerNode(t *testing.T) {
	k, _ := newPotassium()
	nodesInfo := generateNodes(4, 2, 4*int64(units.GiB), int64(units.GiB), 10)
	nodesInfo, total, err := k.SelectMemoryNodes(nodesInfo, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 16, total)
	assert.Equal(t, 4, len(nodesInfo))
	assert.Equal(t, 4, nodesInfo[0].Capacity)
	assert.Equal(t, 4, nodesInfo[1].Capacity)

	res, err := SelectStorageNodes(k, nodesInfo, int64(units.MiB), 4, false)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(res))
	for _, node := range res {
		assert.Equal(t, 1, node.Deploy)
		assert.Equal(t, 3, node.Capacity)
	}
}

func TestSelectStorageNodesWithPreOccupied(t *testing.T) {
	k, _ := newPotassium()
	nodesInfo := generateNodes(4, 2, 4*int64(units.GiB), int64(units.GiB), 10)
	// Set occupied count
	for i := 0; i < 4; i++ {
		nodesInfo[i].Count += i
	}
	nodesInfo, total, err := k.SelectMemoryNodes(nodesInfo, 1.0, 512*int64(units.MiB))
	assert.NoError(t, err)
	assert.Equal(t, 32, total)
	assert.Equal(t, 4, len(nodesInfo))
	for _, node := range nodesInfo {
		assert.Equal(t, 8, node.Capacity)
	}

	res, err := SelectStorageNodes(k, nodesInfo, int64(units.MiB), 6, false)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(res))
	for i, node := range res {
		assert.Equal(t, 5+i, node.Capacity)
		assert.Equal(t, 3-i, node.Deploy)
	}
}

func TestSelectStorageNodesAllocEachDivition(t *testing.T) {
	k, _ := newPotassium()
	nodesInfo := generateNodes(4, 2, 4*int64(units.GiB), int64(units.GiB), 10)
	nodesInfo, total, err := k.SelectMemoryNodes(nodesInfo, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 16, total)
	assert.Equal(t, 4, len(nodesInfo))
	for _, node := range nodesInfo {
		assert.Equal(t, 4, node.Capacity)
	}

	res, err := SelectStorageNodes(k, nodesInfo, int64(units.MiB), 2, true)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(res))
	for _, node := range res {
		assert.Equal(t, 2, node.Deploy)
		assert.Equal(t, 2, node.Capacity)
	}
}

func TestSelectStorageNodesCapacityLessThanMemory(t *testing.T) {
	k, _ := newPotassium()
	nodesInfo := generateNodes(2, 2, 4*int64(units.GiB), int64(units.GiB), 10)
	nodesInfo, total, err := k.SelectMemoryNodes(nodesInfo, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 8, total)
	assert.Equal(t, 2, len(nodesInfo))
	assert.Equal(t, 4, nodesInfo[0].Capacity)
	assert.Equal(t, 4, nodesInfo[1].Capacity)

	res, err := SelectStorageNodes(k, nodesInfo, int64(units.GiB), 2, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 1, res[0].Deploy)
	assert.Equal(t, 1, res[1].Deploy)
	assert.Equal(t, 0, res[0].Capacity)
	assert.Equal(t, 0, res[1].Capacity)
}

func TestSelectStorageNodesNotEnough(t *testing.T) {
	k, _ := newPotassium()
	nodesInfo := generateNodes(1, 2, 4*int64(units.GiB), int64(units.MiB), 10)
	nodesInfo, total, err := k.SelectMemoryNodes(nodesInfo, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 4, total)
	assert.Equal(t, 1, len(nodesInfo))
	assert.Equal(t, 4, nodesInfo[0].Capacity)

	res, err := SelectStorageNodes(k, nodesInfo, int64(units.GiB), 1, false)
	assert.Equal(t, types.ErrInsufficientStorage, err)
	assert.Nil(t, res)
}

func TestSelectStorageNodesSequence(t *testing.T) {
	k, _ := newPotassium()
	nodesInfo := generateNodes(2, 4, 8*int64(units.GiB), 2*int64(units.GiB), 10)
	mem := 512 * int64(units.MiB)
	nodesInfo, total, err := k.SelectMemoryNodes(nodesInfo, 1.0, mem)
	assert.NoError(t, err)
	assert.Equal(t, 32, total)
	assert.Equal(t, 2, len(nodesInfo))
	assert.Equal(t, 16, nodesInfo[0].Capacity)
	assert.Equal(t, 16, nodesInfo[1].Capacity)

	stor := int64(units.GiB)
	res, err := SelectStorageNodes(k, nodesInfo, stor, 1, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 1, res[0].Deploy)
	assert.Equal(t, 1, res[0].Capacity)
	assert.Equal(t, 0, res[0].Count)
	assert.Equal(t, 0, res[1].Deploy)
	assert.Equal(t, 2, res[1].Capacity)
	assert.Equal(t, 0, res[1].Count)

	refreshPod(res, mem, stor)
	assert.Equal(t, 1, res[0].Count)
	assert.Equal(t, 0, res[1].Count)

	res, total, err = k.SelectMemoryNodes(res, 1.0, mem)
	assert.NoError(t, err)
	assert.Equal(t, 31, total)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 15, res[0].Capacity)
	assert.Equal(t, 16, res[1].Capacity)
	lesserResourceNodeName := res[0].Name

	res, err = SelectStorageNodes(k, res, int64(units.GiB), 2, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))

	getLess := func(nodes []types.NodeInfo) (lesser int, greater int) {
		if res[0].Name == lesserResourceNodeName {
			greater = 1
		} else {
			lesser = 1
		}
		return
	}
	i, j := getLess(res)
	assert.Equal(t, 1, res[i].Count)
	assert.Equal(t, 0, res[j].Count)
	assert.Equal(t, 0, res[i].Deploy)
	assert.Equal(t, 2, res[j].Deploy)
	assert.Equal(t, 1, res[i].Capacity)
	assert.Equal(t, 0, res[j].Capacity)

	refreshPod(res, mem, stor)
	assert.Equal(t, 2, res[0].Count)
	assert.Equal(t, 1, res[1].Count)

	res, total, err = k.SelectMemoryNodes(res, 1.0, mem)
	assert.NoError(t, err)
	assert.Equal(t, 29, total)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 14, res[0].Capacity)
	assert.Equal(t, 15, res[1].Capacity)
	assert.Equal(t, 2, res[0].Count)
	assert.Equal(t, 1, res[1].Count)

	res, err = SelectStorageNodes(k, res, int64(units.GiB), 1, false)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, 1, res[0].Count)
	assert.Equal(t, 1, res[0].Deploy)
	assert.Equal(t, 0, res[0].Capacity)
}

func SelectStorageNodes(k *Potassium, nodesInfo []types.NodeInfo, storage int64, need int, each bool) ([]types.NodeInfo, error) {
	switch nodesInfo, total, err := k.SelectStorageNodes(nodesInfo, storage); {
	case err != nil:
		return nil, err
	case each:
		return k.EachDivision(nodesInfo, need, 0)
	default:
		return k.CommonDivision(nodesInfo, need, total)
	}
}
