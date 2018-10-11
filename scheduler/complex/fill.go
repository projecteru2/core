package complexscheduler

import (
	"fmt"
	"sort"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// FillPlan deploy container each node
func FillPlan(nodesInfo []types.NodeInfo, need, limit int) ([]types.NodeInfo, error) {
	nodesInfoLength := len(nodesInfo)
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Count > nodesInfo[j].Count })
	p := sort.Search(nodesInfoLength, func(i int) bool { return nodesInfo[i].Count < need })
	if p == nodesInfoLength {
		return nil, types.ErrAlreadyFilled
	}
	nodesInfo = nodesInfo[p:]
	if limit > 0 {
		nodesInfo = nodesInfo[:limit]
	}
	for i := range nodesInfo {
		diff := need - nodesInfo[i].Count
		if nodesInfo[i].Capacity < diff {
			return nil, types.NewDetailedErr(types.ErrInsufficientRes,
				fmt.Sprintf("node %s cannot alloc a fill node plan", nodesInfo[i].Name))
		}
		nodesInfo[i].Deploy = diff
		nodesInfo[i].Capacity -= diff
	}
	log.Debugf("[FillPlan] nodesInfo: %v", nodesInfo)
	return nodesInfo, nil
}
