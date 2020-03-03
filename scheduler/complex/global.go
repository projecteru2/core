package complexscheduler

import (
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// GlobalDivisionPlan 基于全局资源配额
func GlobalDivisionPlan(nodesInfo []types.NodeInfo, need int) ([]types.NodeInfo, error) {
	nodesInfo = scoreSort(nodesInfo)
	length := len(nodesInfo)
	i := 0

	for need > 0 {
		p := i
		deploy := 0
		delta := 0.0
		if i < length-1 {
			delta = utils.Round(nodesInfo[i+1].CPURate + nodesInfo[i+1].MemRate + nodesInfo[i+1].StorageRate - nodesInfo[i].CPURate + nodesInfo[i].MemRate + nodesInfo[i].StorageRate)
			i++
		}
		for j := 0; j <= p && need > 0 && delta >= 0; j++ {
			// 减枝
			if nodesInfo[j].Capacity == 0 {
				continue
			}
			cost := utils.Round(nodesInfo[j].CPURate + nodesInfo[j].MemRate + nodesInfo[j].StorageRate)
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
	log.Debugf("[GlobalDivisionPlan] nodesInfo: %v", nodesInfo)
	return nodesInfo, nil
}
