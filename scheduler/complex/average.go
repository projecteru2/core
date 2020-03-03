package complexscheduler

import (
	"fmt"
	"sort"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// AveragePlan deploy container each node
func AveragePlan(nodesInfo []types.NodeInfo, need, limit int) ([]types.NodeInfo, error) {
	log.Debugf("[AveragePlan] need %d limit %d", need, limit)
	nodesInfoLength := len(nodesInfo)
	if nodesInfoLength < limit {
		return nil, types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("node len %d cannot alloc an average node plan", nodesInfoLength))
	}
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Capacity < nodesInfo[j].Capacity })
	p := sort.Search(nodesInfoLength, func(i int) bool { return nodesInfo[i].Capacity >= need })
	if p == nodesInfoLength {
		return nil, types.ErrInsufficientCap
	}
	nodesInfo = scoreSort(nodesInfo[p:])
	if limit > 0 {
		nodesInfo = nodesInfo[:limit]
	}
	for i := range nodesInfo {
		nodesInfo[i].Deploy = need
		nodesInfo[i].Capacity -= need
	}

	log.Debugf("[AveragePlan] nodesInfo: %v", nodesInfo)
	return nodesInfo, nil
}
