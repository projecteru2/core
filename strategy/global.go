package strategy

import (
	"fmt"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// GlobalPlan 基于全局资源配额
// 尽量使得资源消耗平均
func GlobalPlan(nodesInfo []types.NodeInfo, need, total int, resourceType types.ResourceType) ([]types.NodeInfo, error) {
	if total < need {
		return nil, types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("need: %d, vol: %d", need, total))
	}
	nodesInfo = scoreSort(nodesInfo, resourceType)
	length := len(nodesInfo)
	i := 0

	for need > 0 {
		p := i
		deploy := 0
		delta := 0.0
		if i < length-1 {
			delta = utils.Round(nodesInfo[i+1].GetResourceUsage(resourceType) - nodesInfo[i].GetResourceUsage(resourceType))
			i++
		}
		for j := 0; j <= p && need > 0 && delta >= 0; j++ {
			// 减枝
			if nodesInfo[j].Capacity == 0 {
				continue
			}
			cost := utils.Round(nodesInfo[j].GetResourceRate(resourceType))
			deploy = int(delta / cost)
			if deploy == 0 {
				deploy = 1
			}
			if deploy > nodesInfo[j].Capacity {
				deploy = nodesInfo[j].Capacity
			}
			if deploy > need {
				deploy = need
			}
			nodesInfo[j].Deploy += deploy
			nodesInfo[j].Capacity -= deploy
			need -= deploy
		}
	}
	// 这里 need 一定会为 0 出来，因为 volTotal 保证了一定大于 need
	// 这里并不需要再次排序了，理论上的排序是基于资源使用率得到的 Deploy 最终方案
	log.Debugf("[GlobalPlan] resource: %v, nodesInfo: %v", resourceType, nodesInfo)
	return nodesInfo, nil
}
