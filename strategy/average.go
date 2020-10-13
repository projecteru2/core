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
func AveragePlan(strategyInfos []types.StrategyInfo, need, total, limit int, resourceType types.ResourceType) (map[string]*types.DeployInfo, error) {
	log.Debugf("[AveragePlan] need %d limit %d", need, limit)
	nodesInfoLength := len(strategyInfos)
	if nodesInfoLength < limit {
		return nil, types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("node len %d < limit, cannot alloc an average node plan", nodesInfoLength))
	}
	sort.Slice(strategyInfos, func(i, j int) bool { return strategyInfos[i].Capacity < strategyInfos[j].Capacity })
	p := sort.Search(nodesInfoLength, func(i int) bool { return strategyInfos[i].Capacity >= need })
	if p == nodesInfoLength {
		return nil, types.ErrInsufficientCap
	}
	strategyInfos = scoreSort(strategyInfos[p:], resourceType)
	if limit > 0 {
		strategyInfos = strategyInfos[:limit]
	}
	deployMap := make(map[string]*types.DeployInfo)
	for i, strategyInfo := range strategyInfos {
		strategyInfos[i].Capacity -= need
		if _, ok := deployMap[strategyInfo.Nodename]; !ok {
			deployMap[strategyInfo.Nodename] = &types.DeployInfo{}
		}
		deployMap[strategyInfo.Nodename].Deploy += need
	}

	log.Debugf("[AveragePlan] resource: %v, strategyInfos: %v", resourceType, strategyInfos)
	return deployMap, nil
}
