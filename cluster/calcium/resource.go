package calcium

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"

	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
)

func (c *calcium) AllocMemoryResource(opts *types.DeployOptions) (map[string]int, error) {
	lock, err := c.Lock(opts.Podname, 30)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()

	cpuandmem, _, err := c.getCPUAndMem(opts.Podname, opts.Nodename, 1.0)
	if err != nil {
		return nil, err
	}

	nodesInfo := utils.GetNodesInfo(cpuandmem)
	log.Debugf("Input opts.CPUQuota: %f", opts.CPUQuota)
	cpuQuota := int(opts.CPUQuota * float64(utils.CpuPeriodBase))
	log.Debugf("Tranfered cpuQuota: %d", cpuQuota)
	nodesInfo, err = c.store.MakeDeployStatus(opts, nodesInfo)
	if err != nil {
		return nil, err
	}

	plan, err := utils.AllocContainerPlan(nodesInfo, cpuQuota, opts.Memory, opts.Count) // 还是以 Bytes 作单位， 不转换了
	if err != nil {
		return nil, err
	}

	// 并发扣除所需资源
	wg := sync.WaitGroup{}
	wg.Add(len(plan))
	for nodename, connum := range plan {
		go func(nodename string, connum int) {
			wg.Done()
			memoryTotal := opts.Memory * int64(connum)
			c.store.UpdateNodeMem(opts.Podname, nodename, memoryTotal, "-")
		}(nodename, connum)
	}
	wg.Wait()
	return plan, nil
}

func (c *calcium) getCPUAndMem(podname, nodename string, quota float64) (map[string]types.CPUAndMem, []*types.Node, error) {
	result := make(map[string]types.CPUAndMem)
	var err error
	var nodes []*types.Node
	if nodename == "" {
		nodes, err = c.ListPodNodes(podname, false)
		if err != nil {
			return result, nil, err
		}
	} else {
		n, err := c.GetNode(podname, nodename)
		if err != nil {
			return result, nil, err
		}
		nodes = append(nodes, n)
	}

	if quota == 0 { // 因为要考虑quota=0.5这种需求，所以这里有点麻烦
		nodes = filterNodes(nodes, true)
	} else {
		nodes = filterNodes(nodes, false)
	}

	if len(nodes) == 0 {
		err := fmt.Errorf("No available nodes")
		return result, nil, err
	}

	result = makeCPUAndMem(nodes)
	return result, nodes, nil
}
