package calcium

import (
	"sync"

	"github.com/projecteru2/core/types"
)

const (
	GITLAB        = "gitlab"
	GITHUB        = "github"
	COPY_FAILED   = "failed"
	COPY_OK       = "ok"
	CpuPeriodBase = 100000
	CpuShareBase  = 102400

	restartAlways = "always"
	minMemory     = 4194304
	root          = "root"
	maxPuller     = 10
)

//NodeContainers store containers and node info
type NodeContainers map[*types.Node][]*types.Container

//NodeCPUMap store cpu and node info
type NodeCPUMap map[*types.Node][]types.CPUMap

//CPUNodeContainers store cpu and nodecontainers
type CPUNodeContainers map[float64]NodeContainers

//CPUNodeContainersMap store cpu and nodecpumap
type CPUNodeContainersMap map[float64]NodeCPUMap

type imageBucket struct {
	sync.Mutex
	data map[string]map[string]struct{}
}

func newImageBucket() *imageBucket {
	return &imageBucket{data: make(map[string]map[string]struct{})}
}

func (ib *imageBucket) Add(podname, image string) {
	ib.Lock()
	defer ib.Unlock()

	if _, ok := ib.data[podname]; !ok {
		ib.data[podname] = make(map[string]struct{})
	}
	ib.data[podname][image] = struct{}{}
}

func (ib *imageBucket) Dump() map[string][]string {
	r := make(map[string][]string)
	for podname, imageMap := range ib.data {
		images := []string{}
		for image := range imageMap {
			images = append(images, image)
		}
		r[podname] = images
	}
	return r
}
