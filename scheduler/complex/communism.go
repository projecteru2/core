package complexscheduler

import (
	"sort"

	"github.com/projecteru2/core/types"
)

// 吃我一记共产主义大锅饭
func CommunismDivisionPlan(arg []types.NodeInfo, need, volTotal int) ([]types.NodeInfo, error) {
	sort.Slice(arg, func(i, j int) bool { return arg[i].Count < arg[j].Count })
	length := len(arg)
	i := 0

	var deploy, differ int
	for need > 0 && volTotal > 0 {
		p := i
		deploy = 0
		differ = 1
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
