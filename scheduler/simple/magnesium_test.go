package simplescheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/types"
)

func TestRandomNode(t *testing.T) {
	m := &Magnesium{}
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

func TestSelectNodes(t *testing.T) {
	m := &Magnesium{}
	_, err := m.SelectNodes(map[string]types.CPUMap{}, 1, 1)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "No nodes provide to choose some")

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

	_, err = m.SelectNodes(nodes, 2, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, err = m.SelectNodes(nodes, 3, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, err = m.SelectNodes(nodes, 1, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	r, err := m.SelectNodes(nodes, 1, 2)
	assert.NoError(t, err)
	for nodename, cpus := range r {
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		assert.Equal(t, len(cpus), 1)

		cpu := cpus[0]
		assert.Equal(t, cpu.Total(), 10)
	}

	_, err = m.SelectNodes(nodes, 1, 4)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	r, err = m.SelectNodes(nodes, 1, 2)
	assert.NoError(t, err)
	for nodename, cpus := range r {
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		assert.Equal(t, len(cpus), 1)

		cpu := cpus[0]
		assert.Equal(t, cpu.Total(), 10)
	}

	_, err = m.SelectNodes(nodes, 1, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")
}
