package complexscheduler

import (
	"sort"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// AveragePlan deploy container each node
func AveragePlan(nodesInfo []types.NodeInfo, need int) ([]types.NodeInfo, error) {
	nodesInfoLength := len(nodesInfo)
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Capacity < nodesInfo[j].Capacity })
	p := sort.Search(nodesInfoLength, func(i int) bool { return nodesInfo[i].Capacity >= need })
	if p == nodesInfoLength {
		return nil, types.ErrInsufficientCap
	}
	nodesInfo = nodesInfo[p:]
	for i := range nodesInfo {
		nodesInfo[i].Deploy = need
		nodesInfo[i].Capacity -= need
	}

	log.Debugf("[AveragePlan] nodesInfo: %v", nodesInfo)
	return nodesInfo, nil
}
