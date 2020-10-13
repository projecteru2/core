package scheduler

import (
	"math"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

var (
	scheduler Scheduler
)

// Scheduler is a scheduler is used to determine which nodes are we gonna use.
// `types.CPUMap` represents the CPU label and remaining quota.
// `nodes` represents node name and the corresponding CPUMap.
type Scheduler interface {
	// select one node from nodes, returns nodename
	// typically used to build image
	MaxIdleNode(nodes []*types.Node) (*types.Node, error)
	SelectStorageNodes(nodesInfo []types.NodeInfo, storage int64) ([]types.NodeInfo, int, error)
	SelectMemoryNodes(nodesInfo []types.NodeInfo, quota float64, memory int64) ([]types.NodeInfo, int, error)
	// select nodes from nodes, return a list of nodenames and the corresponding cpumap, and also the changed nodes with remaining cpumap
	// quota and number must be given, typically used to determine where to deploy
	SelectCPUNodes(nodesInfo []types.NodeInfo, quota float64, memory int64) ([]types.NodeInfo, map[string][]types.CPUMap, int, error)
	// select nodes from nodes, return a list a nodenames and the corresponding volumemap
	SelectVolumeNodes(nodeInfo []types.NodeInfo, vbs types.VolumeBindings) ([]types.NodeInfo, map[string][]types.VolumePlan, int, error)
}

// InitSchedulerV1 .
func InitSchedulerV1(s Scheduler) {
	scheduler = s
}

// GetSchedulerV1 .
func GetSchedulerV1() (Scheduler, error) {
	if scheduler == nil {
		return nil, errors.Errorf("potassium not initiated")
	}
	return scheduler, nil
}

// SelectNodes .
func SelectNodes(resourceRequests []types.ResourceRequest, nodeMap map[string]*types.Node) (planMap map[types.ResourceType]types.ResourcePlans, total int, scheduleTypes types.ResourceType, err error) {
	total = math.MaxInt16
	subTotal := 0
	planMap = make(map[types.ResourceType]types.ResourcePlans)
	nodesInfo := getNodesInfo(nodeMap)
	log.Debugf("[SelectNode] nodesInfo: %+v", nodesInfo)
	for _, req := range resourceRequests {
		scheduler := req.MakeScheduler()
		if planMap[req.Type()], subTotal, err = scheduler(nodesInfo); err != nil {
			return
		}
		total = utils.Min(total, subTotal)

		// calculate schedule type
		if req.Type()&types.ResourceCPUBind != 0 {
			scheduleTypes |= types.ResourceCPU
		}
		if req.Type()&types.ResourceScheduledVolume != 0 {
			scheduleTypes |= types.ResourceVolume
		}
	}

	if scheduleTypes == 0 {
		scheduleTypes = types.ResourceMemory
	}
	return
}

func getNodesInfo(nodes map[string]*types.Node) []types.NodeInfo {
	result := []types.NodeInfo{}
	for _, node := range nodes {
		nodeInfo := types.NodeInfo{
			Name:          node.Name,
			CPUMap:        node.CPU,
			VolumeMap:     node.Volume,
			InitVolumeMap: node.InitVolume,
			MemCap:        node.MemCap,
			StorageCap:    node.StorageCap,
			Capacity:      0,
		}
		result = append(result, nodeInfo)
	}
	return result
}
