package calcium

import (
	"sync"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

const (
	restartAlways = "always"
	minMemory     = types.MByte * 4
	root          = "root"
	maxPuller     = 10
)

type nodeContainers map[*types.Node][]*types.Container

type nodeCPUMap map[*types.Node][]types.CPUMap

type cpuMemNodeContainers map[float64]map[int64]nodeContainers

type cpuMemNodeContainersMap map[float64]map[int64]nodeCPUMap

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
	log.Debugf("[ImageBucket] Dump %v", r)
	return r
}
