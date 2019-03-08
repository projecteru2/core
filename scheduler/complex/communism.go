package complexscheduler

import (
	"sort"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// CommunismDivisionPlan 吃我一记共产主义大锅饭
func CommunismDivisionPlan(arg []types.NodeInfo, need int) ([]types.NodeInfo, error) {
	sort.Slice(arg, func(i, j int) bool { return arg[i].Count < arg[j].Count })
	length := len(arg)

	for i := 0; i < length; i++ {
		if need <= 0 {
			break
		}
		req := need
		if i < length-1 {
			req = (arg[i+1].Count - arg[i].Count) * (i + 1)
		}
		if req > need {
			req = need
		}
		for j := 0; j < i+1; j++ {
			deploy := req / (i + 1 - j)
			tail := req % (i + 1 - j)
			d := deploy
			if tail > 0 {
				d++
			}
			if d > arg[j].Capacity {
				d = arg[j].Capacity
			}
			arg[j].Deploy += d
			arg[j].Capacity -= d
			need -= d
			req -= d
		}
	}
	// 这里 need 一定会为 0 出来，因为 volTotal 保证了一定大于 need
	// 这里并不需要再次排序了，理论上的排序是基于 Count 得到的 Deploy 最终方案
	log.Debugf("[CommunismDivisionPlan] nodesInfo: %v", arg)
	return arg, nil
}
