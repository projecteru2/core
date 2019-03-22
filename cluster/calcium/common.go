package calcium

import (
	"github.com/projecteru2/core/types"
)

const (
	restartAlways = "always"
	minMemory     = types.MByte * 4
	root          = "root"
)

type nodeContainers map[*types.Node][]*types.Container

type nodeCPUMap map[*types.Node][]types.CPUMap

type cpuMemNodeContainers map[float64]map[int64]nodeContainers

type cpuMemNodeContainersMap map[float64]map[int64]nodeCPUMap
