package complexscheduler

import (
	"sort"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// GlobalDivisionPlan 基于全局资源配额
func GlobalDivisionPlan(arg []types.NodeInfo, need int) ([]types.NodeInfo, error) {
	sort.Slice(arg, func(i, j int) bool { return arg[i].CPUUsed+arg[i].MemUsage < arg[j].CPUUsed+arg[j].MemUsage })
	length := len(arg)
	i := 0

	for need > 0 {
		p := i
		deploy := 0
		delta := 0.0
		if i < length-1 {
			delta = utils.Round(arg[i+1].CPUUsed + arg[i+1].MemUsage - arg[i].CPUUsed - arg[i].MemUsage)
			i++
		}
		for j := 0; j <= p && need > 0 && delta >= 0; j++ {
			// 减枝
			if arg[j].Capacity == 0 {
				continue
			}
			cost := utils.Round(arg[j].CPURate + arg[j].MemRate)
			deploy = int(delta / cost)
			if deploy == 0 {
				deploy = 1
			}
			if deploy > arg[j].Capacity {
				deploy = arg[j].Capacity
			}
			if deploy > need {
				deploy = need
			}
			arg[j].Deploy += deploy
			arg[j].Capacity -= deploy
			need -= deploy
		}
	}
	// 这里 need 一定会为 0 出来，因为 volTotal 保证了一定大于 need
	// 这里并不需要再次排序了，理论上的排序是基于资源使用率得到的 Deploy 最终方案
	log.Debugf("[GlobalDivisionPlan] nodesInfo: %v", arg)
	return arg, nil
}
