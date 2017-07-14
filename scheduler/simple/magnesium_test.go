package simplescheduler

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/types"
)

func TestRandomNode(t *testing.T) {
	m := &magnesium{}
	_, err := m.RandomNode(map[string]types.CPUMap{})
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "No nodes provide to choose one")

	nodes := map[string]types.CPUMap{
		"node1": types.CPUMap{
			"0": 10,
			"1": 10,
		},
		"node2": types.CPUMap{
			"0": 10,
			"1": 10,
		},
	}

	node, err := m.RandomNode(nodes)
	assert.NoError(t, err)
	assert.Contains(t, []string{"node1", "node2"}, node)

	nodes = map[string]types.CPUMap{
		"node1": types.CPUMap{
			"0": 10,
			"1": 10,
		},
	}

	node, err = m.RandomNode(nodes)
	assert.NoError(t, err)
	assert.Equal(t, "node1", node)
}

func TestSelectCPUNodes(t *testing.T) {
	m := &magnesium{}
	_, _, err := m.SelectCPUNodes([]types.NodeInfo{}, 1, 1)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "No nodes provide to choose some")

	nodes := []types.NodeInfo{
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 10,
				},
				12400000,
			},
			"node1", 0.0, 0, 0, 0,
		},
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 10,
				},
				12400000,
			},
			"node2", 0.0, 0, 0, 0,
		},
	}

	_, _, err = m.SelectCPUNodes(nodes, 2, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, _, err = m.SelectCPUNodes(nodes, 3, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, _, err = m.SelectCPUNodes(nodes, 1, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	r, changed, err := m.SelectCPUNodes(nodes, 1, 2)
	assert.NoError(t, err)
	assert.Equal(t, len(changed), 2)
	for nodename, cpus := range r {
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		assert.Equal(t, len(cpus), 1)

		cpu := cpus[0]
		assert.Equal(t, int(cpu.Total()), 10)
	}

	_, _, err = m.SelectCPUNodes(nodes, 1, 4)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	r, _, err = m.SelectCPUNodes(nodes, 1, 2)
	assert.NoError(t, err)
	for nodename, cpus := range r {
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		assert.Equal(t, len(cpus), 1)

		cpu := cpus[0]
		assert.Equal(t, int(cpu.Total()), 10)
	}

	_, _, err = m.SelectCPUNodes(nodes, 1, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	for _, node := range nodes {
		cupmap := node.CpuMap
		assert.Equal(t, int(cupmap.Total()), 0)
		assert.Equal(t, len(cupmap), 2)
		assert.Equal(t, int(cupmap["0"]), 0)
		assert.Equal(t, int(cupmap["1"]), 0)
	}
}

func TestResultLength(t *testing.T) {
	c := map[string][]types.CPUMap{
		"node1": []types.CPUMap{
			types.CPUMap{"0": 10},
			types.CPUMap{"1": 10},
		},
		"node2": []types.CPUMap{
			types.CPUMap{"0": 10},
		},
	}
	assert.Equal(t, resultLength(c), 3)
}

func TestTotalQuota(t *testing.T) {
	nodes := []types.NodeInfo{
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 0,
				},
				12400000,
			},
			"node1", 0.0, 0, 0, 0,
		},
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 5,
					"1": 10,
				},
				12400000,
			},
			"node2", 0.0, 0, 0, 0,
		},
	}
	assert.Equal(t, totalQuota(nodes), 3)
}

func TestSelectPublicNodes(t *testing.T) {
	m := &magnesium{}
	_, _, err := m.SelectCPUNodes([]types.NodeInfo{}, 1, 1)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "No nodes provide to choose some")

	nodes := []types.NodeInfo{
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 10,
				},
				12400000,
			},
			"node1", 0.0, 0, 0, 0,
		},
		types.NodeInfo{
			types.CPUAndMem{
				types.CPUMap{
					"0": 10,
					"1": 10,
				},
				12400000,
			},
			"node2", 0.0, 0, 0, 0,
		},
	}

	r, changed, err := m.SelectCPUNodes(nodes, 0, 10)
	assert.NoError(t, err)
	assert.Equal(t, resultLength(r), 10)
	assert.Equal(t, len(changed), 2)
	for nodename, cpus := range r {
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		for _, cpu := range cpus {
			assert.Equal(t, int(cpu.Total()), 0)
		}
	}

	for _, node := range nodes {
		nodename := node.Name
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		assert.Equal(t, int(node.CpuMap.Total()), 20)
	}
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
		Name:     "",
		CPURate:  2e9,
		Capacity: 0,
		Count:    0,
		Deploy:   0,
	}

	nodes := []types.NodeInfo{}
	for i := 0; i < count; i++ {
		n.Name = "node" + strconv.Itoa(i)
		nodes = append(nodes, n)
	}

	return nodes
}

func TestSelectMemoryNodes(t *testing.T) {
	// 2 nodes [2 containers per node]
	pod := newPod(2)
	m := &magnesium{}
	res, err := m.SelectMemoryNodes(pod, 10000, 512*1024*1024, 4)
	assert.NoError(t, err)
	for _, node := range res {
		assert.Equal(t, node.Deploy, 2)
	}

	// 4 nodes [1 1 1 1]
	pod = newPod(4)
	res, err = m.SelectMemoryNodes(pod, 10000, 512*1024*1024, 4)
	assert.NoError(t, err)
	for _, node := range res {
		assert.Equal(t, node.Deploy, 1)
	}

	// 4 nodes [2 1 1 1]
	pod = newPod(4)
	res, err = m.SelectMemoryNodes(pod, 10000, 512*1024*1024, 5)
	assert.NoError(t, err)
	assert.Equal(t, res[0].Deploy, 2)

	// 4 nodes [1 1 1 0]
	pod = newPod(4)
	res, err = m.SelectMemoryNodes(pod, 10000, 512*1024*1024, 3)
	assert.NoError(t, err)
	assert.Equal(t, res[3].Deploy, 0)
}

func TestTestSelectMemoryNodesNotEnough(t *testing.T) {
	// 2 nodes [mem not enough]
	pod := newPod(2)
	m := &magnesium{}
	_, err := m.SelectMemoryNodes(pod, 10000, 512*1024*1024, 40)
	assert.Error(t, err)
	if err != nil {
		assert.Equal(t, err.Error(), "[SelectMemoryNodes] Cannot alloc a plan, not enough memory, volume 16, need 40")
	}

	// 2 nodes [mem not enough]
	pod = newPod(2)
	_, err = m.SelectMemoryNodes(pod, 1e9, 5*1024*1024*1024, 1)
	assert.Error(t, err)
	if err != nil {
		assert.Equal(t, err.Error(), "[SelectMemoryNodes] Cannot alloc a plan, not enough memory, volume 0, need 1")
	}

	// 2 nodes [cpu not enough]
	pod = newPod(2)
	_, err = m.SelectMemoryNodes(pod, 1e10, 512*1024*1024, 1)
	assert.Error(t, err)
	if err != nil {
		assert.Equal(t, err.Error(), "[SelectMemoryNodes] Cannot alloc a plan, not enough cpu rate")
	}
}
