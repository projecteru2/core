package client

import (
	"fmt"
	"strings"

	"github.com/projecteru2/core/resource/plugins/cpumem/types"
	"github.com/projecteru2/core/utils"
)

// NewNUMAFromStr .
func NewNUMAFromStr(numa []string) (types.NUMA, error) {
	r := types.NUMA{}
	for index, numaCPUList := range numa {
		nodeID := fmt.Sprintf("%d", index)
		for _, cpuID := range strings.Split(numaCPUList, ",") {
			r[cpuID] = nodeID
		}
	}
	return r, nil
}

// NewNUMAMemoryFromStr .
func NewNUMAMemoryFromStr(numaMemory []string) (types.NUMAMemory, error) {
	r := types.NUMAMemory{}
	for index, nodeMemory := range numaMemory {
		nodeID := fmt.Sprintf("%d", index)
		mem, err := utils.ParseRAMInHuman(nodeMemory)
		if err != nil {
			return r, err
		}
		r[nodeID] = mem
	}
	return r, nil
}
