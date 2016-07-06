package complexscheduler

import (
	"fmt"
	"testing"

	"github.com/coreos/etcd/client"
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

func TestSelectNodes(t *testing.T) {
	cfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}
	cli, _ := client.New(cfg)
	api := client.NewKeysAPI(cli)

	k, merr := NewPotassim(api, "/coretest", 1)
	if merr != nil {
		t.Fatalf("cannot create Potassim instance.", merr)
	}

	_, err := k.SelectNodes(map[string]types.CPUMap{}, 1, 1)
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

	_, err = k.SelectNodes(nodes, 2, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, err = k.SelectNodes(nodes, 3, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	_, err = k.SelectNodes(nodes, 1, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not enough")

	r, err := k.SelectNodes(nodes, 1, 2)
	assert.NoError(t, err)

	for nodename, cpus := range r {
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		// assert.Equal(t, len(cpus), 1)

		cpu := cpus[0]
		assert.Equal(t, cpu.Total(), 10)
	}

	r, err = k.SelectNodes(nodes, 1.3, 1)
	assert.NoError(t, err)

	for nodename, cpus := range r {
		assert.Contains(t, []string{"node1", "node2"}, nodename)
		assert.Equal(t, len(cpus), 1)

		cpu := cpus[0]
		assert.Equal(t, cpu.Total(), 13)
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
		fmt.Println(name)
		for k, v := range res {
			fmt.Println(k, ":", len(v))
		}
		return fmt.Errorf("alloc plan error")
	}
	return nil
}

func TestComplexNodes(t *testing.T) {
	cfg := client.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}
	cli, _ := client.New(cfg)
	api := client.NewKeysAPI(cli)

	// nodes can offer 28 containers.
	nodes := map[string]types.CPUMap{
		"n1": types.CPUMap{ // 2 containers
			"0": 10, "1": 10, "2": 10, "3": 10,
		},
		"n2": types.CPUMap{ // 7 containers
			"0": 10, "1": 10, "2": 10, "3": 10,
			"4": 10, "5": 10, "6": 10, "7": 10,
			"8": 10, "9": 10, "10": 10, "11": 10,
			"12": 10, "13": 10,
		},
		"n3": types.CPUMap{ // 6 containers
			"0": 10, "1": 10, "2": 10, "3": 10,
			"4": 10, "5": 10, "6": 10, "7": 10,
			"8": 10, "9": 10, "10": 10, "11": 10,
		},
		"n4": types.CPUMap{ // 9 containers
			"0": 10, "1": 10, "2": 10, "3": 10,
			"4": 10, "5": 10, "6": 10, "7": 10,
			"8": 10, "9": 10, "10": 10, "11": 10,
			"12": 10, "13": 10, "14": 10, "15": 10,
			"16": 10, "17": 10,
		},
		"n5": types.CPUMap{ // 4 containers
			"0": 10, "1": 10, "2": 10, "3": 10,
			"4": 10, "5": 10, "6": 10, "7": 10,
		},
	}

	k, _ := NewPotassim(api, "coretest2", 1)

	// test1
	res1, err := k.SelectNodes(nodes, 1.7, 7)
	if err != nil {
		t.Fatalf("sth wrong")
	}
	if check := checkAvgPlan(res1, 1, 2, "res1"); check != nil {
		t.Fatalf("something went wrong")
	}

	// test2
	res2, err := k.SelectNodes(nodes, 1.7, 11)
	if check := checkAvgPlan(res2, 1, 4, "res2"); check != nil {
		t.Fatalf("something went wrong")
	}

	// test3
	res3, err := k.SelectNodes(nodes, 1.7, 23)
	if check := checkAvgPlan(res3, 2, 6, "res3"); check != nil {
		t.Fatalf("something went wrong")
	}

	// test4
	_, newErr := k.SelectNodes(nodes, 1.6, 29)
	if newErr == nil {
		t.Fatalf("how to alloc 29 containers when you only have 28?")
	}
}
