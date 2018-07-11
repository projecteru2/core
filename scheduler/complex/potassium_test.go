package complexscheduler

import (
	"fmt"
	"testing"

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

func generateNodes(nums, cores int, memory, shares int64) []types.NodeInfo {
	var name string
	nodes := []types.NodeInfo{}

	for i := 0; i < nums; i++ {
		name = fmt.Sprintf("n%d", i)

		cpumap := types.CPUMap{}
		for j := 0; j < cores; j++ {
			coreName := fmt.Sprintf("%d", j)
			cpumap[coreName] = shares
		}
		cpuandmem := types.CPUAndMem{
			CpuMap: cpumap,
			MemCap: memory,
		}
		nodeInfo := types.NodeInfo{
			CPUAndMem: cpuandmem,
			Name:      name,
			CPURate:   2e9,
		}
		nodes = append(nodes, nodeInfo)
	}
	return nodes
}

func getNodesCapacity(nodes []types.NodeInfo, cpu float64, shares, maxshare int64) int {
	var res int
	var host *host
	var plan []types.CPUMap

	for _, nodeInfo := range nodes {
		host = newHost(nodeInfo.CPUAndMem.CpuMap, shares)
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

func refreshPod(nodes []types.NodeInfo) {
	for i := range nodes {
		nodes[i].Count += nodes[i].Deploy
		nodes[i].MemCap -= int64(nodes[i].Deploy * 512 * 1024 * 1024)
		nodes[i].Deploy = 0
	}
}

func getComplexNodes() []types.NodeInfo {
	return []types.NodeInfo{
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
}

func getEvenPlanNodes() []types.NodeInfo {
	return []types.NodeInfo{
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
}

func SelectCPUNodes(k *Potassium, nodesInfo []types.NodeInfo, quota float64, memory int64, need int, each bool) (map[string][]types.CPUMap, map[string]types.CPUMap, error) {
	nodesInfo, nodePlans, total, err := k.SelectCPUNodes(nodesInfo, quota, memory)
	if err != nil {
		return nil, nil, err
	}

	if each {
		nodesInfo, err = k.EachDivision(nodesInfo, need, total)
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
			nodeInfo.CpuMap.Sub(cpu)
		}
		changed[nodeInfo.Name] = nodeInfo.CpuMap
	}
	return result, changed, nil
}

func SelectMemoryNodes(k *Potassium, nodesInfo []types.NodeInfo, rate, memory int64, need int, each bool) ([]types.NodeInfo, error) {
	nodesInfo, total, err := k.SelectMemoryNodes(nodesInfo, rate, memory)
	if err != nil {
		return nodesInfo, err
	}
	// Make deploy plan
	if each {
		nodesInfo, err = k.EachDivision(nodesInfo, need, total)
	} else {
		nodesInfo, err = k.CommonDivision(nodesInfo, need, total)
	}
	return nodesInfo, err
}

func TestSelectCPUNodes(t *testing.T) {
	k, _ := newPotassium()

	_, _, err := SelectCPUNodes(k, []types.NodeInfo{}, 1, 1, 1, false)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "No nodes provide to choose some")
	mem := int64(4 * 1024 * 1024 * 1024)

	nodes := generateNodes(2, 2, mem, 10)
	_, _, err = SelectCPUNodes(k, nodes, 0.5, 1, 1, false)
	assert.NoError(t, err)

	_, _, err = SelectCPUNodes(k, nodes, 2, 1, 3, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, _, err = SelectCPUNodes(k, nodes, 3, 1, 2, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, _, err = SelectCPUNodes(k, nodes, 1, 1, 5, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	// new round test
	nodes = generateNodes(2, 2, mem, 10)
	r, re, err := SelectCPUNodes(k, nodes, 1, 1, 2, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(r))
	assert.Equal(t, 2, len(re))

	for nodename, cpus := range r {
		assert.Contains(t, []string{"n0", "n1"}, nodename)
		// assert.Equal(t, len(cpus), 1)
		cpu := cpus[0]
		assert.Equal(t, cpu.Total(), int64(10))
	}

	// SelectCPUNodes 里有一些副作用, 粗暴地拿一个新的来测试吧
	// 下面也是因为这个
	nodes = generateNodes(2, 2, mem, 10)
	r, _, err = SelectCPUNodes(k, nodes, 1.3, 1, 2, false)
	assert.NoError(t, err)

	for nodename, cpus := range r {
		assert.Contains(t, []string{"n0", "n1"}, nodename)
		assert.Equal(t, len(cpus), 1)

		cpu := cpus[0]
		assert.Equal(t, cpu.Total(), int64(13))
	}
}

func TestSelectCPUNodesWithMemoryLimit(t *testing.T) {
	k, _ := newPotassium()

	// 测试 2 个 Node，每个 CPU 10%，但是内存吃满
	nodes := generateNodes(2, 2, 1024, 10)
	result, _, err := SelectCPUNodes(k, nodes, 0.1, 1024, 1, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result), 2)
	for _, cpus := range result {
		assert.Equal(t, len(cpus), 1)
	}

	// 测试 2 个 Node，内存不足
	nodes = generateNodes(2, 2, 1024, 10)
	_, _, err = SelectCPUNodes(k, nodes, 0.1, 1025, 1, true)
	assert.Contains(t, err.Error(), "enough")

	// 测试 need 超过 each node 的 capacity
	nodes = generateNodes(2, 2, 1024, 10)
	_, _, err = SelectCPUNodes(k, nodes, 0.1, 1024, 2, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enough capacity")
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
	assert.Equal(t, err.Error(), "quota must positive")

	//test6
	nodes = getComplexNodes()
	res6, _, err := SelectCPUNodes(k, nodes, 1, 1, 2, true)
	assert.NoError(t, err)
	assert.Equal(t, len(res6), 5)
}

func TestCPUOverSell(t *testing.T) {
	coreCfg := newConfig()
	coreCfg.Scheduler.ShareBase = 5
	k, err := New(coreCfg)
	if err != nil {
		t.Fatalf("Create Potassim error: %v", err)
	}

	//test1
	nodes := getComplexNodes()
	_, _, err = SelectCPUNodes(k, nodes, 1.7, 1, 3, false)
	assert.NoError(t, err)

	//TODO 增加其他测试吧，这里应该是没问题的
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
	res2, rem2, err := SelectCPUNodes(k, pod2, 1.7, 1, 3, false)
	if check := checkAvgPlan(res2, 1, 1, "res2"); check != nil {
		t.Fatalf("something went wront")
	}
	assert.Equal(t, len(rem2), 3)

	pod3 := getEvenPlanNodes()
	res3, rem3, err := SelectCPUNodes(k, pod3, 1.7, 1, 8, false)
	if check := checkAvgPlan(res3, 2, 2, "res3"); check != nil {
		t.Fatalf("something went wront")
	}
	assert.Equal(t, len(rem3), 4)

	pod4 := getEvenPlanNodes()
	res4, rem4, err := SelectCPUNodes(k, pod4, 1.7, 1, 10, false)
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
	res1, _, err := SelectCPUNodes(k, pod, 1.7, 1, 7, false)
	assert.NoError(t, err)
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

	res2, changed2, err := SelectCPUNodes(k, newpod, 1.7, 1, 4, false)
	assert.NoError(t, err)
	assert.Equal(t, len(res2), len(changed2))
	checkAvgPlan(res2, 2, 2, "new test 2")
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
		hugePod := generateNodes(count, 24, 137438953472, 10)
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
	var rate int64 = 1024
	for i := 0; i < b.N; i++ {
		// 24 core, 128G memory, 10 pieces per core
		hugePod := generateNodes(count, 24, 137438953472, 10)
		b.StartTimer()
		r, err := SelectMemoryNodes(k, hugePod, rate, memory, need, false)
		b.StopTimer()
		assert.NoError(b, err)
		assert.Equal(b, len(r), count)
	}
}

// Test SelectMemoryNodes
func TestSelectMemoryNodes(t *testing.T) {
	// 2 nodes [2 containers per node]
	mem := int64(4 * 1024 * 1024 * 1024)
	pod := generateNodes(2, 2, mem, 10)
	k, _ := newPotassium()
	res, err := SelectMemoryNodes(k, pod, 10000, 512*1024*1024, 4, false)
	assert.NoError(t, err)
	for _, node := range res {
		assert.Equal(t, node.Deploy, 2)
	}

	// 4 nodes [1 container per node]
	pod = generateNodes(4, 2, mem, 10)
	res, err = SelectMemoryNodes(k, pod, 10000, 512*1024*1024, 1, false)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy, 1)

	// 4 nodes [1 container per node]
	pod = generateNodes(4, 2, mem, 10)
	res, err = SelectMemoryNodes(k, pod, 10000, 512*1024*1024, 4, false)
	assert.NoError(t, err)
	for _, node := range res {
		assert.Equal(t, node.Deploy, 1)
	}

	// 4 nodes
	pod = generateNodes(4, 2, mem, 10)
	for i := 0; i < 4; i++ {
		pod[i].Count += i
	}
	res, err = SelectMemoryNodes(k, pod, 10000, 512*1024*1024, 6, false)
	assert.NoError(t, err)
	for i, node := range res {
		assert.Equal(t, node.Deploy, 3-i)
	}

	pod = generateNodes(1, 2, mem, 10)
	_, err = SelectMemoryNodes(k, pod, 10000, 0, 10, false)
	assert.Equal(t, err.Error(), "memory must positive")

	// test each
	pod = generateNodes(4, 2, mem, 10)
	each := 2
	res, err = SelectMemoryNodes(k, pod, 1000, 1024, each, true)
	for i := range res {
		assert.Equal(t, res[i].Deploy, each)
	}
}

func TestSelectMemoryNodesNotEnough(t *testing.T) {
	mem := int64(4 * 1024 * 1024)
	// 2 nodes [mem not enough]
	pod := generateNodes(2, 2, mem*1024, 10)
	k, _ := newPotassium()
	_, err := SelectMemoryNodes(k, pod, 10000, 512*1024*1024, 40, false)
	assert.Error(t, err)
	if err != nil {
		assert.Equal(t, err.Error(), "Not enough resource need: 40, vol: 16")
	}

	// 2 nodes [mem not enough]
	pod = generateNodes(2, 2, mem, 10)
	_, err = SelectMemoryNodes(k, pod, 1e9, 5*1024*1024*1024, 1, false)
	assert.Error(t, err)
	if err != nil {
		assert.Equal(t, err.Error(), "Cannot alloc a plan, not enough memory")
	}

	// 2 nodes [cpu not enough]
	pod = generateNodes(2, 2, mem, 10)
	_, err = SelectMemoryNodes(k, pod, 1e10, 512*1024*1024, 1, false)
	assert.Error(t, err)
	if err != nil {
		assert.Equal(t, err.Error(), "Cannot alloc a plan, not enough cpu rate")
	}
}

func TestSelectMemoryNodesSequence(t *testing.T) {
	pod := generateNodes(2, 2, 4*1024*1024*1024, 10)
	k, _ := newPotassium()
	res, err := SelectMemoryNodes(k, pod, 10000, 512*1024*1024, 1, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node0" {
			assert.Equal(t, node.Deploy, 1)
		}
	}

	refreshPod(res)
	res, err = SelectMemoryNodes(k, res, 10000, 512*1024*1024, 1, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node1" {
			assert.Equal(t, node.Deploy, 1)
		}
	}

	refreshPod(res)
	res, err = SelectMemoryNodes(k, res, 10000, 512*1024*1024, 4, false)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy, 2)
	assert.Equal(t, res[1].Deploy, 2)

	refreshPod(res)
	res, err = SelectMemoryNodes(k, res, 10000, 512*1024*1024, 3, false)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy+res[1].Deploy, 3)
	assert.Equal(t, res[0].Deploy-res[1].Deploy, 1)

	refreshPod(res)
	res, err = SelectMemoryNodes(k, res, 10000, 512*1024*1024, 40, false)
	assert.Error(t, err)
	if err != nil {
		assert.Equal(t, err.Error(), "Not enough resource need: 40, vol: 7")
	}

	// new round
	pod = generateNodes(2, 2, 4*1024*1024*1024, 10)
	res, err = SelectMemoryNodes(k, pod, 10000, 512*1024*1024, 1, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node0" {
			assert.Equal(t, node.Deploy, 1)
		}
	}
	refreshPod(res)
	res, err = SelectMemoryNodes(k, res, 10000, 512*1024*1024, 2, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node1" {
			assert.Equal(t, node.Deploy, 2)
		}
	}
	refreshPod(res)
	res, err = SelectMemoryNodes(k, res, 10000, 512*1024*1024, 5, false)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy+res[0].Count, 4)
	assert.Equal(t, res[1].Deploy+res[1].Count, 4)
}

func TestSelectMemoryNodesGiven(t *testing.T) {
	pod := generateNodes(4, 2, 4*1024*1024*1024, 10)
	for i := 0; i < 3; i++ {
		pod[i].Count++
	}

	k, _ := newPotassium()
	res, err := SelectMemoryNodes(k, pod, 10000, 512*1024*1024, 2, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "n3" {
			assert.Equal(t, node.Deploy, 2)
			continue
		}
		assert.Equal(t, node.Deploy, 0)
	}
}
