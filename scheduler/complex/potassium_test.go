package complexscheduler

import (
	"errors"
	"fmt"
	"sort"
	"testing"

	"math"

	"github.com/docker/go-units"
	pkgerrors "github.com/pkg/errors"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/resources/cpumem"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/resources/volume"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
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
	scheduler.InitSchedulerV1(potassium)
	return potassium, nil
}

func generateNodes(nums, cores int, memory, storage int64, shares int) []resourcetypes.ScheduleInfo {
	return utils.GenerateScheduleInfos(nums, cores, memory, storage, shares)
}

func getNodesCapacity(nodes []resourcetypes.ScheduleInfo, cpu float64, shares, maxshare int) int {
	var res int
	var host *host
	var plan []types.CPUMap

	for _, scheduleInfo := range nodes {
		host = newHost(scheduleInfo.CPU, shares)
		plan = host.distributeOneRation(cpu, maxshare)
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

func refreshPod(nodes []resourcetypes.ScheduleInfo, deployMap map[string]int, memory, storage int64) {

	for i := range nodes {
		nodes[i].MemCap -= int64(deployMap[nodes[i].Name]) * memory
		nodes[i].StorageCap -= int64(deployMap[nodes[i].Name]) * storage
	}
}

func getComplexNodes() []resourcetypes.ScheduleInfo {
	return []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 2 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n1",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 7 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
					"12": 10, "13": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n2",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 6 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n3",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 9 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
					"12": 10, "13": 10, "14": 10, "15": 10,
					"16": 10, "17": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n4",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 4 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n5",
			},
		},
	}
}

func getEvenPlanNodes() []resourcetypes.ScheduleInfo {
	return []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 4 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n1",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 5 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n2",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 6 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10, "10": 10, "11": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n3",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 5 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
					"8": 10, "9": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n4",
			},
		},
	}
}

func getNodeMapFromscheduleInfos(scheduleInfos []resourcetypes.ScheduleInfo) map[string]*types.Node {
	nodeMap := map[string]*types.Node{}
	for _, scheduleInfo := range scheduleInfos {
		nodeMap[scheduleInfo.Name] = &types.Node{
			NodeMeta: types.NodeMeta{
				MemCap:     scheduleInfo.MemCap,
				CPU:        scheduleInfo.CPU,
				StorageCap: scheduleInfo.StorageCap,
				Name:       scheduleInfo.Name,
				Volume:     scheduleInfo.Volume,
				InitVolume: scheduleInfo.InitVolume,
			},
		}
	}
	return nodeMap
}

func getInfosFromscheduleInfos(scheduleInfos []resourcetypes.ScheduleInfo, planMap []resourcetypes.ResourcePlans, countMap map[string]int) (strategyInfos []strategy.Info) {
	if countMap == nil {
		countMap = map[string]int{}
	}
	for _, scheduleInfo := range scheduleInfos {
		capacity := math.MaxInt64
		for _, v := range planMap {
			capacity = utils.Min(capacity, v.Capacity()[scheduleInfo.Name])
		}
		if scheduleInfo.Capacity > 0 {
			capacity = utils.Min(capacity, scheduleInfo.Capacity)
		}
		if capacity == math.MaxInt64 {
			capacity = 0
		}
		if capacity == 0 {
			continue
		}
		strategyInfos = append(strategyInfos, strategy.Info{
			Nodename: scheduleInfo.Name,
			Count:    countMap[scheduleInfo.Name],
			Capacity: capacity,
		})
	}
	return strategyInfos
}

func newDeployOptions(need int, each bool) *types.DeployOptions {
	opts := &types.DeployOptions{
		DeployStrategy: strategy.Auto,
		Count:          need,
	}
	if each {
		opts.DeployStrategy = strategy.Each
	}
	return opts
}

func SelectCPUNodes(k *Potassium, scheduleInfos []resourcetypes.ScheduleInfo, countMap map[string]int, quota float64, memory int64, need int, each bool) (map[string][]types.CPUMap, map[string]types.CPUMap, error) {
	rrs, err := resources.MakeRequests(types.ResourceOptions{CPUQuotaLimit: quota, MemoryLimit: memory, CPUBind: true})
	if err != nil {
		return nil, nil, err
	}
	nodeMap := getNodeMapFromscheduleInfos(scheduleInfos)
	sType, total, planMap, err := resources.SelectNodesByResourceRequests(rrs, nodeMap)
	if err != nil {
		return nil, nil, err
	}

	deployMap, err := strategy.Deploy(newDeployOptions(need, each), getInfosFromscheduleInfos(scheduleInfos, planMap, countMap), total, sType)
	if err != nil {
		return nil, nil, err
	}
	result := make(map[string][]types.CPUMap)
	changed := make(map[string]types.CPUMap)
	for nodename, deploy := range deployMap {
		for _, plan := range planMap {
			if CPUPlan, ok := plan.(cpumem.ResourcePlans); ok {
				result[nodename] = CPUPlan.CPUPlans[nodename][:deploy]
				plan.ApplyChangesOnNode(nodeMap[nodename], utils.Range(deploy)...)
				changed[nodename] = nodeMap[nodename].CPU
			}
		}
	}
	return result, changed, nil
}

func SelectMemoryNodes(k *Potassium, scheduleInfos []resourcetypes.ScheduleInfo, countMap map[string]int, rate float64, memory int64, need int, each bool) ([]resourcetypes.ScheduleInfo, map[string]int, error) {
	rrs, err := resources.MakeRequests(types.ResourceOptions{CPUQuotaLimit: rate, MemoryLimit: memory})
	if err != nil {
		return nil, nil, err
	}
	sType, total, planMap, err := resources.SelectNodesByResourceRequests(rrs, getNodeMapFromscheduleInfos(scheduleInfos))
	if err != nil {
		return nil, nil, err
	}

	deployMap, err := strategy.Deploy(newDeployOptions(need, each), getInfosFromscheduleInfos(scheduleInfos, planMap, countMap), total, sType)
	if err != nil {
		return nil, nil, err
	}
	return scheduleInfos, deployMap, nil
}

func TestSelectCPUNodes(t *testing.T) {
	k, _ := newPotassium()
	memory := 4 * int64(units.GiB)

	_, _, err := SelectCPUNodes(k, []resourcetypes.ScheduleInfo{}, nil, 1, 1, 1, false)
	assert.True(t, errors.Is(err, types.ErrZeroNodes))

	_, _, err = SelectCPUNodes(k, []resourcetypes.ScheduleInfo{}, nil, 1, -1, 1, false)
	assert.EqualError(t, err, "limit or request less than 0: bad `Memory` value")

	nodes := generateNodes(2, 2, memory, 0, 10)
	_, _, err = SelectCPUNodes(k, nodes, nil, 0.5, 1, 1, false)
	assert.NoError(t, err)

	_, _, err = SelectCPUNodes(k, nodes, nil, 2, 1, 3, false)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))
	assert.Contains(t, err.Error(), "need: 3, vol: 1")

	_, _, err = SelectCPUNodes(k, nodes, nil, 3, 1, 2, false)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))

	_, _, err = SelectCPUNodes(k, nodes, nil, 1, 1, 5, false)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))

	// new round test
	nodes = generateNodes(2, 2, memory, 0, 10)
	r, re, err := SelectCPUNodes(k, nodes, nil, 1, 1, 2, false)
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
	nodes = generateNodes(2, 2, memory, 0, 10)
	r, _, err = SelectCPUNodes(k, nodes, nil, 1.3, 1, 2, false)
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

	_, _, _, err := k.SelectCPUNodes([]resourcetypes.ScheduleInfo{}, 0, 0)
	assert.Error(t, err)

	// 测试 2 个 Node，每个 CPU 10%，但是内存吃满
	nodes := generateNodes(2, 2, 1024, 0, 10)
	result, _, err := SelectCPUNodes(k, nodes, nil, 0.1, 1024, 1, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result), 2)
	for _, cpus := range result {
		assert.Equal(t, len(cpus), 1)
	}

	// 测试 2 个 Node，内存不足
	nodes = generateNodes(2, 2, 1024, 0, 10)
	_, _, err = SelectCPUNodes(k, nodes, nil, 0.1, 1025, 1, true)
	assert.EqualError(t, err, types.ErrInsufficientRes.Error())

	// 测试 need 超过 each node 的 capacity
	nodes = generateNodes(2, 2, 1024, 0, 10)
	_, _, err = SelectCPUNodes(k, nodes, nil, 0.1, 1024, 2, true)
	assert.EqualError(t, err, types.ErrInsufficientCap.Error())
}

func TestRecurrence(t *testing.T) {
	// 利用相同线上数据复现线上出现的问题

	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 0, "10": 0, "7": 0, "8": 10, "9": 10, "13": 0, "14": 0, "15": 10, "2": 10, "5": 10, "11": 0, "12": 0, "4": 0, "1": 0, "3": 10, "6": 0},
				MemCap: 12 * int64(units.GiB),
				Name:   "c2-node-26",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"6": 10, "10": 0, "13": 0, "14": 10, "2": 0, "7": 0, "1": 0, "11": 0, "15": 0, "8": 10, "0": 0, "3": 0, "4": 0, "5": 0, "9": 10, "12": 0},
				MemCap: 12 * int64(units.GiB),
				Name:   "c2-node-27",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"13": 0, "14": 0, "15": 0, "4": 10, "9": 0, "1": 0, "10": 0, "12": 10, "5": 10, "6": 10, "8": 10, "0": 0, "11": 0, "2": 10, "3": 0, "7": 0},
				MemCap: 12 * int64(units.GiB),
				Name:   "c2-node-28",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"15": 0, "3": 10, "0": 0, "10": 0, "13": 0, "7": 10, "8": 0, "9": 10, "12": 10, "2": 10, "4": 10, "1": 0, "11": 0, "14": 10, "5": 10, "6": 10},
				MemCap: 12 * int64(units.GiB),
				Name:   "c2-node-29",
			},
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
	scheduler.InitSchedulerV1(k)

	// test1
	nodes := getComplexNodes()
	res1, changed1, err := SelectCPUNodes(k, nodes, nil, 1.7, 1, 7, false)
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
	res2, changed2, err := SelectCPUNodes(k, nodes, nil, 1.7, 1, 11, false)
	if err != nil {
		t.Fatalf("something went wrong")
	}
	if check := checkAvgPlan(res2, 2, 3, "res2"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(changed2), len(res2))

	// test3
	nodes = getComplexNodes()
	res3, changed3, err := SelectCPUNodes(k, nodes, nil, 1.7, 1, 23, false)
	assert.NoError(t, err)
	if check := checkAvgPlan(res3, 2, 6, "res3"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(changed3), len(res3))

	// test4
	nodes = getComplexNodes()
	_, _, newErr := SelectCPUNodes(k, nodes, nil, 1.6, 1, 29, false)
	if newErr == nil {
		t.Fatalf("how to alloc 29 workloads when you only have 28?")
	}

	//test5
	nodes = getComplexNodes()
	res6, _, err := SelectCPUNodes(k, nodes, nil, 1, 1, 2, true)
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
	scheduler.InitSchedulerV1(k)

	// oversell
	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100, "4": 100, "5": 100},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}

	_, _, err = SelectCPUNodes(k, nodes, nil, 1.7, 1, 3, false)
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
	scheduler.InitSchedulerV1(k)

	// oversell
	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 300, "1": 300},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}

	r, c, err := SelectCPUNodes(k, nodes, nil, 2, 1, 3, false)
	assert.NoError(t, err)
	assert.Equal(t, r["nodes1"][0]["0"], int64(100))
	assert.Equal(t, r["nodes1"][0]["1"], int64(100))
	assert.Equal(t, c["nodes1"]["0"], int64(0))
	assert.Equal(t, c["nodes1"]["1"], int64(0))

	// oversell fragment
	nodes = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 300},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}

	_, _, err = SelectCPUNodes(k, nodes, nil, 0.5, 1, 6, false)
	assert.NoError(t, err)

	// one core oversell
	nodes = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 300},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}

	_, _, err = SelectCPUNodes(k, nodes, nil, 1, 1, 2, false)
	assert.NoError(t, err)

	// balance
	nodes = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 100, "1": 200, "2": 300},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}
	_, c, err = SelectCPUNodes(k, nodes, nil, 1, 1, 2, false)
	assert.NoError(t, err)
	assert.Equal(t, c["nodes1"]["0"], int64(0))
	assert.Equal(t, c["nodes1"]["1"], int64(100))

	// complex
	nodes = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 50, "1": 100, "2": 300, "3": 70, "4": 200, "5": 30, "6": 230},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}
	_, _, err = SelectCPUNodes(k, nodes, nil, 1.7, 1, 2, false)
	assert.NoError(t, err)

	nodes = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 70, "1": 100, "2": 400},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}
	_, c, err = SelectCPUNodes(k, nodes, nil, 1.3, 1, 4, false)
	assert.NoError(t, err)
	assert.Equal(t, c["nodes1"]["0"], int64(10))
	assert.Equal(t, c["nodes1"]["1"], int64(40))
	assert.Equal(t, c["nodes1"]["2"], int64(0))
}

func TestCPUOverSellAndStableFragmentCore(t *testing.T) {
	coreCfg := newConfig()
	coreCfg.Scheduler.ShareBase = 100
	k, err := New(coreCfg)
	if err != nil {
		t.Fatalf("Create Potassim error: %v", err)
	}
	scheduler.InitSchedulerV1(k)

	// oversell
	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 300, "1": 300},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}

	_, _, err = SelectCPUNodes(k, nodes, nil, 1.7, 1, 1, false)
	assert.NoError(t, err)

	// stable fragment core
	nodes = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 230, "1": 200},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}
	res, changed, err := SelectCPUNodes(k, nodes, nil, 1.7, 1, 1, false)
	println(res)
	assert.NoError(t, err)
	assert.Equal(t, changed["nodes1"]["0"], int64(160))
	nodes[0].CPU = changed["nodes1"]
	_, changed, err = SelectCPUNodes(k, nodes, nil, 0.3, 1, 1, false)
	assert.NoError(t, err)
	assert.Equal(t, changed["nodes1"]["0"], int64(130))
	assert.Equal(t, changed["nodes1"]["1"], int64(100))

	// complex node
	nodes = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 230, "1": 80, "2": 300, "3": 200},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}
	_, changed, err = SelectCPUNodes(k, nodes, nil, 1.7, 1, 2, false)
	assert.NoError(t, err)
	assert.Equal(t, changed["nodes1"]["0"], int64(160))
	assert.Equal(t, changed["nodes1"]["1"], int64(10))

	// consume full core
	nodes = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 70, "1": 50, "2": 100, "3": 100, "4": 100},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}
	_, changed, err = SelectCPUNodes(k, nodes, nil, 1.7, 1, 2, false)
	assert.NoError(t, err)
	assert.Equal(t, changed["nodes1"]["0"], int64(0))
	assert.Equal(t, changed["nodes1"]["1"], int64(50))

	// consume less fragment core
	nodes = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"0": 70, "1": 50, "2": 90},
				MemCap: 12 * int64(units.GiB),
				Name:   "nodes1",
			},
		},
	}
	_, changed, err = SelectCPUNodes(k, nodes, nil, 0.5, 1, 2, false)
	assert.NoError(t, err)
	assert.Equal(t, changed["nodes1"]["0"], int64(20))
	assert.Equal(t, changed["nodes1"]["1"], int64(0))
	assert.Equal(t, changed["nodes1"]["2"], int64(90))
}

func TestEvenPlan(t *testing.T) {
	k, merr := newPotassium()
	if merr != nil {
		t.Fatalf("Create Potassim error: %v", merr)
	}

	// nodes -- n1: 2, n2: 2
	pod1 := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{
					"0": 10, "1": 10, "2": 10, "3": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "node1",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{
					"0": 10, "1": 10, "2": 10, "3": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "node2",
			},
		},
	}

	res1, rem1, err := SelectCPUNodes(k, pod1, nil, 1.3, 1, 2, false)
	if err != nil {
		t.Fatalf("sth wrong")
	}
	if check := checkAvgPlan(res1, 1, 1, "res1"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(rem1), 2)

	// nodes -- n1: 4, n2: 5, n3:6, n4: 5
	pod2 := getEvenPlanNodes()
	res2, rem2, _ := SelectCPUNodes(k, pod2, nil, 1.7, 1, 3, false)
	if check := checkAvgPlan(res2, 1, 1, "res2"); check != nil {
		t.Fatalf("something went wront")
	}
	assert.Equal(t, len(rem2), 3)

	pod3 := getEvenPlanNodes()
	res3, rem3, _ := SelectCPUNodes(k, pod3, nil, 1.7, 1, 8, false)
	if check := checkAvgPlan(res3, 2, 2, "res3"); check != nil {
		t.Fatalf("something went wront")
	}
	assert.Equal(t, len(rem3), 4)

	pod4 := getEvenPlanNodes()
	res4, rem4, _ := SelectCPUNodes(k, pod4, nil, 1.7, 1, 10, false)
	if check := checkAvgPlan(res4, 2, 3, "res4"); check != nil {
		t.Fatalf("something went wrong")
	}
	assert.Equal(t, len(rem4), 4)
}

func TestSpecialCase(t *testing.T) {
	pod := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 4 workloads
					"0": 10, "1": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n1",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 5 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n2",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 6 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n3",
			},
		},
	}

	k, _ := newPotassium()
	res1, _, err := SelectCPUNodes(k, pod, nil, 1.7, 1, 7, false)
	assert.NoError(t, err)
	checkAvgPlan(res1, 1, 3, "new test 2")

	newpod := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 4 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n1",
			},
		},
		{
			NodeMeta: types.NodeMeta{
				CPU: types.CPUMap{ // 4 workloads
					"0": 10, "1": 10, "2": 10, "3": 10,
					"4": 10, "5": 10, "6": 10, "7": 10,
				},
				MemCap: 12 * int64(units.GiB),
				Name:   "n2",
			},
		},
	}

	res2, changed2, err := SelectCPUNodes(k, newpod, nil, 1.7, 1, 4, false)
	assert.NoError(t, err)
	assert.Equal(t, len(res2), len(changed2))
	checkAvgPlan(res2, 2, 2, "new test 2")
}

func TestGetPodVol(t *testing.T) {
	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				CPU:    types.CPUMap{"15": 0, "3": 10, "0": 0, "10": 0, "13": 0, "7": 10, "8": 0, "9": 10, "12": 10, "2": 10, "4": 10, "1": 0, "11": 0, "14": 10, "5": 10, "6": 10},
				MemCap: 12 * int64(units.GiB),
				Name:   "c2-node-26",
			},
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
		r, c, err := SelectCPUNodes(k, hugePod, nil, cpu, 1, need, false)
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
	// 128M per workload
	var memory int64 = 1024 * 1024 * 128
	// Max vol is 128G/128M * 10000 nodes
	var need = 10240000
	for i := 0; i < b.N; i++ {
		// 24 core, 128G memory, 10 pieces per core
		hugePod := generateNodes(count, 24, 128*int64(units.GiB), 0, 10)
		b.StartTimer()
		r, _, err := SelectMemoryNodes(k, hugePod, nil, 1, memory, need, false)
		b.StopTimer()
		assert.NoError(b, err)
		assert.Equal(b, len(r), count)
	}
}

// Test SelectMemoryNodes
func TestSelectMemoryNodes(t *testing.T) {
	// 2 nodes [2 workloads per node]
	memory := 4 * int64(units.GiB)
	pod := generateNodes(2, 2, memory, 0, 10)
	k, _ := newPotassium()
	// nega memory
	_, _, err := SelectMemoryNodes(k, pod, nil, 1.0, -1, 4, false)
	assert.Error(t, err)

	cpus := 1.0
	res, deployMap, err := SelectMemoryNodes(k, pod, nil, cpus, 512*int64(units.MiB), 4, false)
	assert.NoError(t, err)
	for _, node := range res {
		assert.Equal(t, deployMap[node.Name], 2)
	}

	// 4 nodes [1 workload on the first node]
	pod = generateNodes(4, 2, memory, 0, 10)
	res, deployMap, err = SelectMemoryNodes(k, pod, nil, cpus, 512*int64(units.MiB), 1, false)
	assert.NoError(t, err)
	assert.Equal(t, deployMap[res[0].Name], 1)

	// 4 nodes [1 workload per node]
	pod = generateNodes(4, 2, memory, 0, 10)
	res, deployMap, err = SelectMemoryNodes(k, pod, nil, cpus, 512*int64(units.MiB), 4, false)
	assert.NoError(t, err)
	for _, node := range res {
		assert.Equal(t, deployMap[node.Name], 1)
	}

	// 4 nodes
	pod = generateNodes(4, 2, memory, 0, 10)
	countMap := map[string]int{}
	for i := 0; i < 4; i++ {
		countMap[pod[i].Name] += i
	}
	res, deployMap, err = SelectMemoryNodes(k, pod, countMap, cpus, 512*int64(units.MiB), 6, false)
	assert.NoError(t, err)
	for i, node := range res {
		assert.Equal(t, deployMap[node.Name], 3-i)
	}

	pod = generateNodes(1, 2, memory, 0, 10)
	_, _, err = SelectMemoryNodes(k, pod, nil, cpus, -1, 10, false)
	assert.EqualError(t, err, "limit or request less than 0: bad `Memory` value")

	// test each
	pod = generateNodes(4, 2, memory, 0, 10)
	each := 2
	res, deployMap, _ = SelectMemoryNodes(k, pod, nil, 1000, 1024, each, true)
	for i := range res {
		assert.Equal(t, deployMap[res[i].Name], each)
	}
}

func TestSelectMemoryNodesNotEnough(t *testing.T) {
	memory := 4 * int64(units.MiB)
	// 2 nodes [memory not enough]
	pod := generateNodes(2, 2, 4*int64(units.GiB), 0, 10)
	k, _ := newPotassium()
	_, _, err := SelectMemoryNodes(k, pod, nil, 1, 512*int64(units.MiB), 40, false)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))
	assert.Contains(t, err.Error(), "need: 40, vol: 16")

	// 2 nodes [memory not enough]
	pod = generateNodes(2, 2, memory, 0, 10)
	_, _, err = SelectMemoryNodes(k, pod, nil, 1, 5*int64(units.GiB), 1, false)
	assert.Equal(t, pkgerrors.Cause(err), types.ErrInsufficientMEM)

	// 2 nodes [cpu not enough]
	pod = generateNodes(2, 2, memory, 0, 10)
	_, _, err = SelectMemoryNodes(k, pod, nil, 1e10, 512*int64(units.MiB), 1, false)
	assert.Equal(t, pkgerrors.Cause(err), types.ErrInsufficientCPU)
}

func TestSelectMemoryNodesSequence(t *testing.T) {
	pod := generateNodes(2, 2, 4*int64(units.GiB), 0, 10)
	k, _ := newPotassium()
	cpu := 1.0
	mem := 512 * int64(units.MiB)
	res, deployMap, err := SelectMemoryNodes(k, pod, nil, cpu, mem, 1, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node0" {
			assert.Equal(t, deployMap[node.Name], 1)
		}
	}

	refreshPod(res, deployMap, mem, 0)
	res, deployMap, err = SelectMemoryNodes(k, res, nil, cpu, mem, 1, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node1" {
			assert.Equal(t, deployMap[node.Name], 1)
		}
	}

	refreshPod(res, deployMap, mem, 0)
	res, deployMap, err = SelectMemoryNodes(k, res, nil, cpu, mem, 4, false)
	assert.NoError(t, err)
	assert.Equal(t, deployMap[res[0].Name], 2)
	assert.Equal(t, deployMap[res[1].Name], 2)

	refreshPod(res, deployMap, mem, 0)
	res, deployMap, err = SelectMemoryNodes(k, res, nil, cpu, mem, 3, false)
	assert.NoError(t, err)
	assert.Equal(t, deployMap[res[0].Name]+deployMap[res[1].Name], 3)
	assert.Equal(t, deployMap[res[0].Name]-deployMap[res[1].Name], 1)

	refreshPod(res, deployMap, mem, 0)
	_, _, err = SelectMemoryNodes(k, res, nil, cpu, mem, 40, false)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))
	assert.Contains(t, err.Error(), "need: 40, vol: 7")

	// new round
	pod = generateNodes(2, 2, 4*int64(units.GiB), 0, 10)
	res, deployMap, err = SelectMemoryNodes(k, pod, nil, cpu, mem, 1, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node0" {
			assert.Equal(t, deployMap[node.Name], 1)
		}
	}
	refreshPod(res, deployMap, mem, 0)
	res, deployMap, err = SelectMemoryNodes(k, res, nil, cpu, mem, 2, false)
	assert.NoError(t, err)
	for _, node := range res {
		if node.Name == "node1" {
			assert.Equal(t, deployMap[node.Name], 2)
		}
	}
	refreshPod(res, deployMap, mem, 0)
	res, deployMap, err = SelectMemoryNodes(k, res, nil, cpu, mem, 5, false)
	assert.NoError(t, err)
	finalCounts := []int{deployMap[res[0].Name], deployMap[res[1].Name]}
	sort.Ints(finalCounts)
	assert.ElementsMatch(t, []int{2, 3}, finalCounts)

}

func TestSelectMemoryNodesGiven(t *testing.T) {
	pod := generateNodes(4, 2, 4*int64(units.GiB), 0, 10)
	countMap := map[string]int{}
	for i := 0; i < 3; i++ {
		countMap[pod[i].Name]++

	}

	k, _ := newPotassium()
	_, deployMap, err := SelectMemoryNodes(k, pod, countMap, 1.0, 512*int64(units.MiB), 2, false)
	assert.NoError(t, err)
	finalCounts := []int{}
	for _, node := range pod {
		finalCounts = append(finalCounts, countMap[node.Name]+deployMap[node.Name])
	}
	sort.Ints(finalCounts)
	assert.ElementsMatch(t, []int{1, 1, 1, 2}, finalCounts)

}

func TestMaxIdleNode(t *testing.T) {
	n1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:       "n1",
			CPU:        types.CPUMap{"0": 20},
			InitCPU:    types.CPUMap{"0": 100},
			MemCap:     30,
			InitMemCap: 100,
		},
	}
	n2 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:       "n1",
			CPU:        types.CPUMap{"0": 30},
			InitCPU:    types.CPUMap{"0": 100},
			MemCap:     10,
			InitMemCap: 100,
		},
	}
	k, _ := newPotassium()
	_, err := k.MaxIdleNode([]*types.Node{})
	assert.Error(t, err)
	node, err := k.MaxIdleNode([]*types.Node{n1, n2})
	assert.NoError(t, err)
	assert.Equal(t, node.Name, n2.Name)
}

func TestSelectStorageNodesMultipleDeployedPerNode(t *testing.T) {
	k, _ := newPotassium()
	emptyNode := []resourcetypes.ScheduleInfo{}
	_, r, err := k.SelectStorageNodes(emptyNode, -1)
	assert.Zero(t, r)
	assert.Error(t, err)
	_, r, err = k.SelectStorageNodes(emptyNode, 0)
	assert.Equal(t, r, math.MaxInt64)
	assert.NoError(t, err)
	scheduleInfos := generateNodes(2, 2, 4*int64(units.GiB), 8*int64(units.GiB), 10)
	scheduleInfos, total, err := k.SelectMemoryNodes(scheduleInfos, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 8, total)
	assert.Equal(t, 2, len(scheduleInfos))
	assert.Equal(t, 4, scheduleInfos[0].Capacity)
	assert.Equal(t, 4, scheduleInfos[1].Capacity)

	res, deployMap, err := SelectStorageNodes(k, scheduleInfos, nil, int64(units.GiB), 4, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 2, deployMap[res[0].Name])
	assert.Equal(t, 2, deployMap[res[1].Name])
	assert.Equal(t, 2, res[0].Capacity)
	assert.Equal(t, 2, res[1].Capacity)
}

func TestSelectStorageNodesDeployedOnFirstNode(t *testing.T) {
	k, _ := newPotassium()
	scheduleInfos := generateNodes(2, 2, 4*int64(units.GiB), int64(units.GiB), 10)
	scheduleInfos, total, err := k.SelectMemoryNodes(scheduleInfos, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 8, total)
	assert.Equal(t, 2, len(scheduleInfos))
	assert.Equal(t, 4, scheduleInfos[0].Capacity)
	assert.Equal(t, 4, scheduleInfos[1].Capacity)

	res, deployMap, err := SelectStorageNodes(k, scheduleInfos, nil, int64(units.MiB), 1, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 1, deployMap[res[0].Name])
	assert.Equal(t, 0, deployMap[res[1].Name])
	assert.Equal(t, 3, res[0].Capacity)
	assert.Equal(t, 4, res[1].Capacity)
}

func TestSelectStorageNodesOneDeployedPerNode(t *testing.T) {
	k, _ := newPotassium()
	scheduleInfos := generateNodes(4, 2, 4*int64(units.GiB), int64(units.GiB), 10)
	scheduleInfos, total, err := k.SelectMemoryNodes(scheduleInfos, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 16, total)
	assert.Equal(t, 4, len(scheduleInfos))
	assert.Equal(t, 4, scheduleInfos[0].Capacity)
	assert.Equal(t, 4, scheduleInfos[1].Capacity)

	res, deployMap, err := SelectStorageNodes(k, scheduleInfos, nil, int64(units.MiB), 4, false)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(res))
	for _, node := range res {
		assert.Equal(t, 1, deployMap[node.Name])
		assert.Equal(t, 3, node.Capacity)
	}
}

func TestSelectStorageNodesWithPreOccupied(t *testing.T) {
	k, _ := newPotassium()
	scheduleInfos := generateNodes(4, 2, 4*int64(units.GiB), int64(units.GiB), 10)
	// Set occupied count
	countMap := map[string]int{}
	for i := 0; i < 4; i++ {
		countMap[scheduleInfos[i].Name] += i
	}
	scheduleInfos, total, err := k.SelectMemoryNodes(scheduleInfos, 1.0, 512*int64(units.MiB))
	assert.NoError(t, err)
	assert.Equal(t, 32, total)
	assert.Equal(t, 4, len(scheduleInfos))
	for _, node := range scheduleInfos {
		assert.Equal(t, 8, node.Capacity)
	}

	res, deployMap, err := SelectStorageNodes(k, scheduleInfos, countMap, int64(units.MiB), 6, false)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(res))
	for i, node := range res {
		assert.Equal(t, 5+i, node.Capacity)
		assert.Equal(t, 3-i, deployMap[node.Name])
	}
}

func TestSelectStorageNodesAllocEachDivition(t *testing.T) {
	k, _ := newPotassium()
	scheduleInfos := generateNodes(4, 2, 4*int64(units.GiB), int64(units.GiB), 10)
	scheduleInfos, total, err := k.SelectMemoryNodes(scheduleInfos, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 16, total)
	assert.Equal(t, 4, len(scheduleInfos))
	for _, node := range scheduleInfos {
		assert.Equal(t, 4, node.Capacity)
	}

	res, deployMap, err := SelectStorageNodes(k, scheduleInfos, nil, int64(units.MiB), 2, true)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(res))
	for _, node := range res {
		assert.Equal(t, 2, deployMap[node.Name])
		assert.Equal(t, 2, node.Capacity)
	}
}

func TestSelectStorageNodesCapacityLessThanMemory(t *testing.T) {
	k, _ := newPotassium()
	scheduleInfos := generateNodes(2, 2, 4*int64(units.GiB), int64(units.GiB), 10)
	scheduleInfos, total, err := k.SelectMemoryNodes(scheduleInfos, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 8, total)
	assert.Equal(t, 2, len(scheduleInfos))
	assert.Equal(t, 4, scheduleInfos[0].Capacity)
	assert.Equal(t, 4, scheduleInfos[1].Capacity)

	res, deployMap, err := SelectStorageNodes(k, scheduleInfos, nil, int64(units.GiB), 2, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 1, deployMap[res[0].Name])
	assert.Equal(t, 1, deployMap[res[1].Name])
	assert.Equal(t, 0, res[0].Capacity)
	assert.Equal(t, 0, res[1].Capacity)
}

func TestSelectStorageNodesNotEnough(t *testing.T) {
	k, _ := newPotassium()
	scheduleInfos := generateNodes(1, 2, 4*int64(units.GiB), int64(units.MiB), 10)
	scheduleInfos, total, err := k.SelectMemoryNodes(scheduleInfos, 1.0, int64(units.GiB))
	assert.NoError(t, err)
	assert.Equal(t, 4, total)
	assert.Equal(t, 1, len(scheduleInfos))
	assert.Equal(t, 4, scheduleInfos[0].Capacity)

	res, _, err := SelectStorageNodes(k, scheduleInfos, nil, int64(units.GiB), 1, false)
	assert.Equal(t, types.ErrInsufficientStorage, pkgerrors.Cause(err))
	assert.Nil(t, res)
}

func TestSelectStorageNodesSequence(t *testing.T) {
	k, _ := newPotassium()
	scheduleInfos := generateNodes(2, 4, 8*int64(units.GiB), 2*int64(units.GiB), 10)
	mem := 512 * int64(units.MiB)
	scheduleInfos, total, err := k.SelectMemoryNodes(scheduleInfos, 1.0, mem)
	assert.NoError(t, err)
	assert.Equal(t, 32, total)
	assert.Equal(t, 2, len(scheduleInfos))
	assert.Equal(t, 16, scheduleInfos[0].Capacity)
	assert.Equal(t, 16, scheduleInfos[1].Capacity)

	stor := int64(units.GiB)
	res, deployMap, err := SelectStorageNodes(k, scheduleInfos, nil, stor, 1, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 1, res[0].Capacity)
	assert.Equal(t, 2, res[1].Capacity)
	counts := []int{deployMap[scheduleInfos[0].Name], deployMap[scheduleInfos[1].Name]}
	sort.Ints(counts)
	assert.ElementsMatch(t, []int{0, 1}, counts)

	refreshPod(res, deployMap, mem, stor)
	countMap := map[string]int{
		res[0].Name: 1,
		res[1].Name: 0,
	}

	res, total, err = k.SelectMemoryNodes(res, 1.0, mem)
	assert.NoError(t, err)
	assert.Equal(t, 31, total)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 15, res[0].Capacity)
	assert.Equal(t, 16, res[1].Capacity)
	lesserResourceNodeName := res[0].Name

	res, deployMap, err = SelectStorageNodes(k, res, countMap, int64(units.GiB), 2, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))

	getLess := func(nodes []resourcetypes.ScheduleInfo) (lesser int, greater int) {
		if res[0].Name == lesserResourceNodeName {
			greater = 1
		} else {
			lesser = 1
		}
		return
	}
	i, j := getLess(res)
	getFinalCounts := func(deployMap, countMap map[string]int) (counts []int) {
		for name, d := range deployMap {
			counts = append(counts, d+countMap[name])
		}
		sort.Ints(counts)
		return
	}
	assert.ElementsMatch(t, []int{1, 2}, getFinalCounts(deployMap, countMap))

	refreshPod(res, deployMap, mem, stor)
	countMap = map[string]int{
		res[i].Name: 1,
		res[j].Name: 2,
	}

	res, total, err = k.SelectMemoryNodes(res, 1.0, mem)
	assert.NoError(t, err)
	assert.Equal(t, 29, total)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 14, res[0].Capacity)
	assert.Equal(t, 15, res[1].Capacity)
}

func SelectStorageNodes(k *Potassium, scheduleInfos []resourcetypes.ScheduleInfo, countMap map[string]int, storage int64, need int, each bool) ([]resourcetypes.ScheduleInfo, map[string]int, error) {
	rrs, err := resources.MakeRequests(types.ResourceOptions{StorageLimit: storage})
	if err != nil {
		return nil, nil, err
	}
	sType, total, planMap, err := resources.SelectNodesByResourceRequests(rrs, getNodeMapFromscheduleInfos(scheduleInfos))
	if err != nil {
		return nil, nil, err
	}

	strategyInfos := getInfosFromscheduleInfos(scheduleInfos, planMap, countMap)
	deployMap, err := strategy.Deploy(newDeployOptions(need, each), strategyInfos, total, sType)
	if err != nil {
		return nil, nil, err
	}
	for i, scheduleInfo := range scheduleInfos {
		for _, si := range strategyInfos {
			if si.Nodename == scheduleInfo.Name {
				scheduleInfos[i].Capacity = si.Capacity - deployMap[si.Nodename]
			}
		}
	}
	return scheduleInfos, deployMap, nil
}

func SelectVolumeNodes(k *Potassium, scheduleInfos []resourcetypes.ScheduleInfo, countMap map[string]int, volumes []string, need int, each bool) (map[string][]types.VolumePlan, map[string]types.VolumeMap, error) {
	rrs, err := resources.MakeRequests(types.ResourceOptions{VolumeLimit: types.MustToVolumeBindings(volumes)})
	if err != nil {
		return nil, nil, err
	}
	nodeMap := getNodeMapFromscheduleInfos(scheduleInfos)
	sType, total, planMap, err := resources.SelectNodesByResourceRequests(rrs, nodeMap)
	if err != nil {
		return nil, nil, err
	}

	deployMap, err := strategy.Deploy(newDeployOptions(need, each), getInfosFromscheduleInfos(scheduleInfos, planMap, countMap), total, sType)
	if err != nil {
		return nil, nil, err
	}
	result := make(map[string][]types.VolumePlan)
	changed := make(map[string]types.VolumeMap)
	for nodename, deploy := range deployMap {
		for _, plan := range planMap {
			if volumePlan, ok := plan.(volume.ResourcePlans); ok {
				result[nodename] = volumePlan.GetPlan(nodename)
				plan.ApplyChangesOnNode(nodeMap[nodename], utils.Range(deploy)...)
				changed[nodename] = nodeMap[nodename].Volume
			}
		}
	}
	return result, changed, nil
}

func TestSelectVolumeNodesNonAuto(t *testing.T) {
	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data0": 1024,
				},
			},
		},
	}

	volumes := []string{
		"/tmp:/tmp:rw:2048",
		"/var/log:/var/log:ro",
		"/data0:/data:rw",
		"/data0:/data",
	}
	res, changed, err := SelectVolumeNodes(k, nodes, nil, volumes, 2, true)
	assert.NoError(t, err)
	assert.Equal(t, len(res["0"]), 0)
	assert.Equal(t, changed["node1"]["/data0"], int64(0))
}

func TestSelectVolumeNodesAutoInsufficient(t *testing.T) {
	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data0": 1024,
					"/data1": 2048,
				},
			},
		},
	}

	volumes := []string{"AUTO:/data:rw:2049"}
	_, _, err := SelectVolumeNodes(k, nodes, nil, volumes, 1, true)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))

	volumes = []string{"AUTO:/data:rw:1024", "AUTO:/dir:rw:1024"}
	_, _, err = SelectVolumeNodes(k, nodes, nil, volumes, 2, true)
	assert.Contains(t, err.Error(), "not enough capacity")
}

func TestSelectVolumeNodesAutoSingle(t *testing.T) {
	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data0": 1024,
					"/data1": 2048,
				},
			},
		},
	}

	volumes := []string{"AUTO:/data:rw:70"}
	res, changed, err := SelectVolumeNodes(k, nodes, nil, volumes, 43, true)
	assert.Nil(t, err)
	assert.Equal(t, len(res["0"]), 43)
	assert.Equal(t, res["0"][0][types.MustToVolumeBinding("AUTO:/data:rw:70")], types.VolumeMap{"/data0": 70})
	assert.Equal(t, changed["0"], types.VolumeMap{"/data0": 44, "/data1": 18})
}

func TestSelectVolumeNodesAutoDouble(t *testing.T) {
	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data0": 1024,
					"/data1": 1025,
				},
			},
		},
		{
			NodeMeta: types.NodeMeta{
				Name: "1",
				Volume: types.VolumeMap{
					"/data0": 2048,
					"/data1": 2049,
				},
			},
		},
	}

	volumes := []string{"AUTO:/data:rw:20", "AUTO:/dir:rw:200"}
	res, changed, err := SelectVolumeNodes(k, nodes, nil, volumes, 5, true)
	assert.Nil(t, err)
	assert.Equal(t, res["0"][4][types.MustToVolumeBinding("AUTO:/data:rw:20")], types.VolumeMap{"/data0": 20})
	assert.Equal(t, res["0"][4][types.MustToVolumeBinding("AUTO:/dir:rw:200")], types.VolumeMap{"/data1": 200})
	assert.Equal(t, res["1"][4][types.MustToVolumeBinding("AUTO:/data:rw:20")], types.VolumeMap{"/data0": 20})
	assert.Equal(t, res["1"][4][types.MustToVolumeBinding("AUTO:/dir:rw:200")], types.VolumeMap{"/data0": 200})
	assert.Equal(t, changed["0"], types.VolumeMap{"/data0": 124, "/data1": 825})
	assert.Equal(t, changed["1"], types.VolumeMap{"/data0": 948, "/data1": 2049})
}

func TestSelectVolumeNodesAutoTriple(t *testing.T) {
	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data1": 1218,
					"/data2": 1219,
					"/data0": 2000,
				},
			},
		},
		{
			NodeMeta: types.NodeMeta{
				Name: "1",
				Volume: types.VolumeMap{
					"/data1": 100,
					"/data2": 10,
					"/data3": 2110,
				},
			},
		},
		{
			NodeMeta: types.NodeMeta{
				Name: "2",
				Volume: types.VolumeMap{
					"/data2": 1001,
					"/data3": 1000,
					"/data4": 1002,
				},
			},
		},
	}

	volumes := []string{
		"AUTO:/data0:rw:1000",
		"AUTO:/data1:rw:10",
		"AUTO:/data2:rw:100",
	}

	res, changed, err := SelectVolumeNodes(k, nodes, nil, volumes, 2, true)
	assert.Nil(t, err)
	assert.Equal(t, res["0"][1][types.MustToVolumeBinding("AUTO:/data0:rw:1000")], types.VolumeMap{"/data2": 1000})
	assert.Equal(t, res["0"][1][types.MustToVolumeBinding("AUTO:/data1:rw:10")], types.VolumeMap{"/data2": 10})
	assert.Equal(t, res["0"][1][types.MustToVolumeBinding("AUTO:/data2:rw:100")], types.VolumeMap{"/data1": 100})

	assert.Equal(t, res["1"][0][types.MustToVolumeBinding("AUTO:/data0:rw:1000")], types.VolumeMap{"/data3": 1000})
	assert.Equal(t, res["1"][0][types.MustToVolumeBinding("AUTO:/data1:rw:10")], types.VolumeMap{"/data2": 10})
	assert.Equal(t, res["1"][0][types.MustToVolumeBinding("AUTO:/data2:rw:100")], types.VolumeMap{"/data1": 100})

	assert.Equal(t, res["2"][1][types.MustToVolumeBinding("AUTO:/data0:rw:1000")], types.VolumeMap{"/data4": 1000})
	assert.Equal(t, res["2"][1][types.MustToVolumeBinding("AUTO:/data1:rw:10")], types.VolumeMap{"/data2": 10})
	assert.Equal(t, res["2"][1][types.MustToVolumeBinding("AUTO:/data2:rw:100")], types.VolumeMap{"/data2": 100})

	assert.Equal(t, changed["0"], types.VolumeMap{"/data1": 8, "/data2": 209, "/data0": 2000})
	assert.Equal(t, changed["1"], types.VolumeMap{"/data1": 0, "/data2": 0, "/data3": 0})
	assert.Equal(t, changed["2"], types.VolumeMap{"/data2": 781, "/data3": 0, "/data4": 2})
}

func TestSelectMonopoly(t *testing.T) {
	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data0": 2000,
					"/data2": 2000,
				},
				InitVolume: types.VolumeMap{
					"/data0": 2001,
					"/data2": 2000,
				},
			},
		},
	}

	volumes := []string{"AUTO:/data:rwm:997"}
	res, changed, err := SelectVolumeNodes(k, nodes, nil, volumes, 1, true)

	assert.Nil(t, err)
	assert.Equal(t, res["0"][0][types.MustToVolumeBinding("AUTO:/data:rwm:997")], types.VolumeMap{"/data2": 2000})
	assert.Equal(t, changed["0"], types.VolumeMap{"/data0": 2000, "/data2": 0})

}

func TestSelectMultipleMonopoly(t *testing.T) {
	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data0": 2000,
					"/data2": 2000,
					"/data3": 3000,
				},
				InitVolume: types.VolumeMap{
					"/data0": 2000,
					"/data2": 2001,
					"/data3": 3000,
				},
			},
		},
	}

	volumes := []string{"AUTO:/data:rom:100", "AUTO:/data1:rom:200"}
	res, changed, err := SelectVolumeNodes(k, nodes, nil, volumes, 2, true)

	assert.Nil(t, err)
	assert.Equal(t, len(res["0"]), 2)
	assert.Equal(t, res["0"][0][types.MustToVolumeBinding(volumes[0])], types.VolumeMap{"/data0": 666})
	assert.Equal(t, res["0"][0][types.MustToVolumeBinding(volumes[1])], types.VolumeMap{"/data0": 1333})
	assert.Equal(t, res["0"][1][types.MustToVolumeBinding(volumes[0])], types.VolumeMap{"/data3": 1000})
	assert.Equal(t, res["0"][1][types.MustToVolumeBinding(volumes[1])], types.VolumeMap{"/data3": 2000})
	assert.Equal(t, changed["0"], types.VolumeMap{"/data0": 1, "/data2": 2000, "/data3": 0})
}

func TestSelectHyperMonopoly(t *testing.T) {
	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data0": 2000,
					"/data2": 2000,
				},
				InitVolume: types.VolumeMap{
					"/data0": 2000,
					"/data2": 2001,
				},
			},
		},
	}

	volumes := []string{
		"AUTO:/data:rom:100", "AUTO:/data1:rmw:200", "AUTO:/data2:m:300",
		"AUTO:/data3:ro:100", "AUTO:/data4:rw:400",
	}
	res, changed, err := SelectVolumeNodes(k, nodes, nil, volumes, 1, true)

	assert.Nil(t, err)
	assert.Equal(t, len(res["0"]), 1)
	assert.Equal(t, res["0"][0][types.MustToVolumeBinding(volumes[0])], types.VolumeMap{"/data0": 333})
	assert.Equal(t, res["0"][0][types.MustToVolumeBinding(volumes[1])], types.VolumeMap{"/data0": 666})
	assert.Equal(t, res["0"][0][types.MustToVolumeBinding(volumes[2])], types.VolumeMap{"/data0": 1000})
	assert.Equal(t, res["0"][0][types.MustToVolumeBinding(volumes[3])], types.VolumeMap{"/data2": 100})
	assert.Equal(t, res["0"][0][types.MustToVolumeBinding(volumes[4])], types.VolumeMap{"/data2": 400})
	assert.Equal(t, changed["0"], types.VolumeMap{"/data0": 1, "/data2": 1500})
}

func TestSelectMonopolyOnMultipleNodes(t *testing.T) {
	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data0": 2000,
					"/data1": 2000,
				},
				InitVolume: types.VolumeMap{
					"/data0": 2001,
					"/data1": 2000,
				},
			},
		},
		{
			NodeMeta: types.NodeMeta{
				Name: "1",
				Volume: types.VolumeMap{
					"/data0": 2000,
					"/data1": 2000,
				},
				InitVolume: types.VolumeMap{
					"/data0": 2000,
					"/data1": 2001,
				},
			},
		},
		{
			NodeMeta: types.NodeMeta{
				Name: "2",
			},
		},
	}

	volumes := []string{"AUTO:/data:rom:100", "AUTO:/data1:wrm:300"}
	res, changed, err := SelectVolumeNodes(k, nodes, nil, volumes, 1, true)

	assert.Nil(t, err)
	assert.Equal(t, res["0"][0][types.MustToVolumeBinding(volumes[0])], types.VolumeMap{"/data1": 500})
	assert.Equal(t, res["0"][0][types.MustToVolumeBinding(volumes[1])], types.VolumeMap{"/data1": 1500})
	assert.Equal(t, res["1"][0][types.MustToVolumeBinding(volumes[0])], types.VolumeMap{"/data0": 500})
	assert.Equal(t, res["1"][0][types.MustToVolumeBinding(volumes[1])], types.VolumeMap{"/data0": 1500})
	assert.Equal(t, changed["0"], types.VolumeMap{"/data0": 2000, "/data1": 0})
	assert.Equal(t, changed["1"], types.VolumeMap{"/data0": 0, "/data1": 2000})
}

func TestSelectMonopolyInsufficient(t *testing.T) {
	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data0": 2000,
				},
				InitVolume: types.VolumeMap{
					"/data0": 2001,
				},
			},
		},
	}

	volumes := []string{"AUTO:/data:m:1"}
	_, _, err := SelectVolumeNodes(k, nodes, nil, volumes, 1, true)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))

	nodes = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data0": 2000,
					"/data1": 2000,
				},
				InitVolume: types.VolumeMap{
					"/data0": 2000,
					"/data1": 2001,
				},
			},
		},
	}

	volumes = []string{"/AUTO:/data:m:200"}
	_, _, err = SelectVolumeNodes(k, nodes, nil, volumes, 2, true)
	assert.True(t, errors.Is(err, types.ErrInsufficientCap))
}

func TestSelectUnlimited(t *testing.T) {
	k, _ := newPotassium()

	nodes := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "0",
				Volume: types.VolumeMap{
					"/data1": 1218,
					"/data2": 1219,
					"/data0": 2000,
				},
			},
		},
		{
			NodeMeta: types.NodeMeta{
				Name: "1",
				Volume: types.VolumeMap{
					"/data1": 100,
					"/data2": 10,
					"/data3": 2110,
				},
			},
		},
		{
			NodeMeta: types.NodeMeta{
				Name: "2",
				Volume: types.VolumeMap{
					"/data2": 1001,
					"/data3": 1000,
					"/data4": 1002,
				},
			},
		},
	}

	volumes := []string{
		"AUTO:/data0:rw:1000",
		"AUTO:/data1:rw:10",
		"AUTO:/data2:rw:100",
		"AUTO:/data3:rw:0",
		"AUTO:/data4:rw:0",
	}

	res, _, _ := SelectVolumeNodes(k, nodes, nil, volumes, 2, true)

	assert.Equal(t, res["0"][1][types.MustToVolumeBinding("AUTO:/data0:rw:1000")], types.VolumeMap{"/data2": 1000})
	assert.Equal(t, res["0"][1][types.MustToVolumeBinding("AUTO:/data1:rw:10")], types.VolumeMap{"/data2": 10})
	assert.Equal(t, res["0"][1][types.MustToVolumeBinding("AUTO:/data2:rw:100")], types.VolumeMap{"/data1": 100})
	assert.Equal(t, res["0"][1][types.MustToVolumeBinding("AUTO:/data3:rw:0")], types.VolumeMap{"/data0": 0})
	assert.Equal(t, res["0"][1][types.MustToVolumeBinding("AUTO:/data4:rw:0")], types.VolumeMap{"/data0": 0})

	assert.Equal(t, res["1"][0][types.MustToVolumeBinding("AUTO:/data0:rw:1000")], types.VolumeMap{"/data3": 1000})
	assert.Equal(t, res["1"][0][types.MustToVolumeBinding("AUTO:/data1:rw:10")], types.VolumeMap{"/data2": 10})
	assert.Equal(t, res["1"][0][types.MustToVolumeBinding("AUTO:/data2:rw:100")], types.VolumeMap{"/data1": 100})
	assert.Equal(t, res["1"][0][types.MustToVolumeBinding("AUTO:/data3:rw:0")], types.VolumeMap{"/data3": 0})
	assert.Equal(t, res["1"][0][types.MustToVolumeBinding("AUTO:/data4:rw:0")], types.VolumeMap{"/data3": 0})

	assert.Equal(t, res["2"][1][types.MustToVolumeBinding("AUTO:/data0:rw:1000")], types.VolumeMap{"/data4": 1000})
	assert.Equal(t, res["2"][1][types.MustToVolumeBinding("AUTO:/data1:rw:10")], types.VolumeMap{"/data2": 10})
	assert.Equal(t, res["2"][1][types.MustToVolumeBinding("AUTO:/data2:rw:100")], types.VolumeMap{"/data2": 100})
	assert.Equal(t, res["2"][1][types.MustToVolumeBinding("AUTO:/data3:rw:0")], types.VolumeMap{"/data4": 0})
	assert.Equal(t, res["2"][1][types.MustToVolumeBinding("AUTO:/data4:rw:0")], types.VolumeMap{"/data4": 0})

}
