package strategy

import (
	"fmt"
	"sort"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// AveragePlan deploy container each node
// 容量够的机器每一台部署 N 个
// need 是每台机器所需总量，limit 是限制节点数
func AveragePlan(nodesInfo []types.NodeInfo, need, limit int, resourceType types.ResourceType) ([]types.NodeInfo, error) {
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
	nodesInfo = scoreSort(nodesInfo[p:], resourceType)
	if limit > 0 {
		nodesInfo = nodesInfo[:limit]
	}
	for i := range nodesInfo {
		nodesInfo[i].Deploy = need
		nodesInfo[i].Capacity -= need
	}

	log.Debugf("[AveragePlan] resource: %v, nodesInfo: %v", resourceType, nodesInfo)
	return nodesInfo, nil
}
