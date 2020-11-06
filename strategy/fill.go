package strategy

import (
	"fmt"
	"sort"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// FillPlan deploy container each node
// 根据之前部署的策略每一台补充到 N 个，超过 N 个忽略
// need 是每台上限, limit 是限制节点数
func FillPlan(strategyInfos []Info, need, total, limit int, resourceType types.ResourceType) (map[string]*types.DeployInfo, error) {
	log.Debugf("[FillPlan] need %d limit %d", need, limit)
	nodesInfoLength := len(strategyInfos)
	if nodesInfoLength < limit {
		return nil, types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("node len %d cannot alloc a fill node plan", nodesInfoLength))
	}
	sort.Slice(strategyInfos, func(i, j int) bool { return strategyInfos[i].Count > strategyInfos[j].Count })
	p := sort.Search(nodesInfoLength, func(i int) bool { return strategyInfos[i].Count < need })
	if p == nodesInfoLength {
		return nil, types.ErrAlreadyFilled
	}
	strategyInfos = scoreSort(strategyInfos[p:], resourceType)
	if limit > 0 && len(strategyInfos) > limit {
		strategyInfos = strategyInfos[:limit]
	}
	deployMap := make(map[string]*types.DeployInfo)
	for i, strategyInfo := range strategyInfos {
		diff := need - strategyInfos[i].Count
		if strategyInfos[i].Capacity < diff {
			return nil, types.NewDetailedErr(types.ErrInsufficientRes,
				fmt.Sprintf("node %s cannot alloc a fill node plan", strategyInfos[i].Nodename))
		}
		strategyInfos[i].Capacity -= diff
		if _, ok := deployMap[strategyInfo.Nodename]; !ok {
			deployMap[strategyInfo.Nodename] = &types.DeployInfo{}
		}
		deployMap[strategyInfo.Nodename].Deploy += diff
	}
	log.Debugf("[FillPlan] resource: %v, strategyInfos: %v", resourceType, strategyInfos)
	return deployMap, nil
}
