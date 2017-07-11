package complexscheduler

import (
	"gitlab.ricebook.net/platform/core/types"
)

func equalDivisionPlan(arg []types.NodeInfo, need, volTotal int) ([]types.NodeInfo, error) {
	length := len(arg)
	i := 0
	for need > 0 && volTotal > 0 {
		p := i
		deploy := 0
		differ := 1
		if i < length-1 {
			differ = arg[i+1].Count - arg[i].Count
			i++
		}
		for j := 0; j <= p && need > 0 && differ > 0; j++ {
			// 减枝
			if arg[j].Capacity == 0 {
				continue
			}
			deploy = differ
			if deploy > arg[j].Capacity {
				deploy = arg[j].Capacity
			}
			if deploy > need {
				deploy = need
			}
			arg[j].Deploy += deploy
			arg[j].Capacity -= deploy
			need -= deploy
			volTotal -= deploy
		}
	}
	// 这里 need 一定会为 0 出来，因为 volTotal 在外层保证了一定大于 need
	return arg, nil
}
